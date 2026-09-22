import os
import json
import asyncio
import httpx
import logging
import math
from typing import List, Dict, Any
from crawl4ai import LLMExtractionStrategy, LLMConfig
from utils.occupation_classifier import OccupationClassifier
from utils.industry_classifier import IndustryClassifier
from utils.thunder_id_client import ThunderIDClient  
from config import BACKEND_BASE_URL, BATCH_SIZE

from crawlers.base_crawler import BaseJobCrawler
from parsers.rooster_parser import RoosterParser
from utils.dedup_utils import JobDuplicationCheck

logger = logging.getLogger(__name__)

class RoosterCrawler(BaseJobCrawler):

    def __init__(self):
        self._parser = RoosterParser()
        self.duplication_checker = JobDuplicationCheck()
        # By creating a singleton for this thunder client we can reused it across the crawler
        self._thunder_client = ThunderIDClient() 

    # There is no error handling for this network call
    async def _fetch_all_jobs(self, async_client: httpx.AsyncClient):
        base_url = "https://api.rooster.jobs/jobSearch/jobs/search"
        limit = 20
        all_jobs = []
        
        # Initial call to get total count
        payload = {"query": [], "limit": limit, "page": 1, "filters": {"country": "Sri Lanka"}}
        response = (await async_client.post(base_url, json=payload)).json()
        # unsafe to access the keys from respose because if the requests returns 429, 500 etc, the result wont be in the required shape.
        total_jobs = response['body']['count']
        total_pages = math.ceil(total_jobs / limit)
        # remove unnecessary commented line
        #total_pages = 1
        
        logger.info(f"Total jobs to fetch: {total_jobs} over {total_pages} pages.")

        # instead of going one by one can we use semaphore concept to trigger a group of requests concurrency? (ex: 5 requests at once)
        # by this way we can improve the performance
        # use response.raise_for_status() after the API call
        for page in range(1, total_pages + 1):
            # user logger instead of the print
            print(f"Fetching page {page}...")
            payload['page'] = page
            # there is no error handling over the network close
            response = (await async_client.post(base_url, json=payload)).json()

            # unsafe to access the keys from respose because if the requests returns 429, 500 etc, the result wont be in the required shape.
            for job in response['body']['data']:
                all_jobs.append(job)
                
            await asyncio.sleep(1)
            
        return all_jobs

    #This funtion starts the crawler and save or update the job after checking whether job already exists or not
    async def crawl_jobs(
        self,
        crawler_run_id: int,
        async_client: httpx.AsyncClient,
        schema: dict,
        instruction: str,
        occupation_classifier: OccupationClassifier,
        industry_classifier: IndustryClassifier,
    ) -> None:

        logger.info("Rooster crawl started.")

        try:
            token = await self._thunder_client.get_access_token()
        except Exception as e:
            logger.error(f"Failed to obtain ThunderID access token: {e}")
            raise
        auth_headers = {"Authorization": f"Bearer {token}"}  
        
        job_data_list = await self._fetch_all_jobs(async_client)

        new_jobs_buffer: List[Dict[str, Any]] = []
        lsh_index_buffer: List[Dict[str, Any]] = []
        updated_jobs_buffer: List[Dict[str, Any]] = []

         # this is duplicated crawlers
        detail_extraction_strategy = LLMExtractionStrategy(
            llm_config=LLMConfig(
                provider="deepseek/deepseek-chat",
                api_token=os.getenv("DEEPSEEK_API_KEY"),
            ),
            instruction=instruction,
            schema=json.dumps(schema),
            extraction_type="schema", 
            apply_chunking=False,          
            extra_args={"base_url": "https://api.deepseek.com", "temperature": 0.0},
        )

        for result in job_data_list:

            temp_payload = self._parser.parse_rule_based_fields(result)

            raw_text = json.dumps(result)

            minhash_sig, lsh_indexes = self.duplication_checker.generate_production_minhash_and_lsh(temp_payload)
            is_duplicate, matched_id = await self.duplication_checker.check_duplicate_via_backend(
                async_client,
                BACKEND_BASE_URL,
                lsh_indexes,
                minhash_sig,
                auth_headers,
                incoming_location=temp_payload.get("location", ""),
            )

            if is_duplicate:
                logger.info(f"Duplicate Match Found: Routing job reference {matched_id} to keep-alive updates.")
                updated_jobs_buffer.append({
                    "job_post_id": matched_id,
                    "crawler_run_id": crawler_run_id,
                })

                if len(updated_jobs_buffer) >= BATCH_SIZE:
                    await async_client.post(f"{BACKEND_BASE_URL}/jobs/batch-update", json={"duplicates": updated_jobs_buffer}, headers=auth_headers,)
                    updated_jobs_buffer.clear()
            else:
                logger.info(f"Unique entry found. Calling LLM to parse entire schema")
                llm_res = await asyncio.to_thread(
                    detail_extraction_strategy.run, url="", sections=[raw_text]
                )

                if llm_res and isinstance(llm_res, list) and len(llm_res) > 0:
                    try:
                        extracted_job = llm_res[0]

                        job_text = f"{extracted_job.get('job_role', '')} {extracted_job.get('job_description', '')}"
                        occupation_group_id = await occupation_classifier.classify(job_text)
                        industry_subclass_id = await industry_classifier.classify(job_text)

                        extracted_job["meta_data"]["crawler_run_id"] = crawler_run_id
                        extracted_job["meta_data"]["minhash_signature"] = minhash_sig
                        extracted_job["meta_data"]["occupation_group_id"] = occupation_group_id
                        extracted_job["meta_data"]["industry_subclass_id"] = industry_subclass_id
                        extracted_job["meta_data"]["source"] = {"source": "Rooster"}

                        new_jobs_buffer.append(extracted_job)

                        for idx_item in lsh_indexes:
                            lsh_index_buffer.append(idx_item)

                    except Exception as e:
                        logger.error(f"Failed to unmarshal LLM response into schema format: {e}")

                if len(new_jobs_buffer) >= BATCH_SIZE:
                    payload = {"new_jobs": new_jobs_buffer, "lsh_indexes": lsh_index_buffer}
                    await async_client.post(f"{BACKEND_BASE_URL}/jobs/batch-save", json=payload, headers=auth_headers,)
                    new_jobs_buffer.clear()
                    lsh_index_buffer.clear()

        if updated_jobs_buffer:
            logger.info(f"Flushing remaining {len(updated_jobs_buffer)} update records to backend.")
            await async_client.post(f"{BACKEND_BASE_URL}/jobs/batch-update", json={"duplicates": updated_jobs_buffer}, headers=auth_headers,)
            updated_jobs_buffer.clear()

        if new_jobs_buffer:
            logger.info(f"Flushing remaining {len(new_jobs_buffer)} insertion records to backend.")
            payload = {"new_jobs": new_jobs_buffer, "lsh_indexes": lsh_index_buffer}
            await async_client.post(f"{BACKEND_BASE_URL}/jobs/batch-save", json=payload, headers=auth_headers,)
            new_jobs_buffer.clear()
            lsh_index_buffer.clear()

        logger.info("Rooster crawl pass concluded.") 