import hashlib
import logging
from typing import List, Dict, Any, Tuple

import httpx
from datasketch import MinHash

logger = logging.getLogger(__name__)

class JobDuplicationCheck:

    # comment fix --> for a given job
    #This function generates the minhash and lsh keys for a give job
    def generate_production_minhash_and_lsh(
        self, job_data: dict, num_perm: int = 128, num_bands: int = 8
    ) -> Tuple[List[int], List[Dict[str, Any]]]:
        text_content = f"{job_data['employer']} {job_data['job_role']} {job_data['location']} {job_data['description']}"
        text_content = " ".join(text_content.lower().split())

        k = 3
        shingles = set(text_content[i:i + k] for i in range(len(text_content) - k + 1))

        m = MinHash(num_perm=num_perm)
        for shingle in shingles:
            m.update(shingle.encode('utf-8'))

        raw_signature = [int(h) for h in m.hashvalues]
        lsh_indexes = []
        hashes_per_band = num_perm // num_bands

        for band_no in range(num_bands):
            start_idx = band_no * hashes_per_band
            end_idx = start_idx + hashes_per_band
            band_hashes = m.hashvalues[start_idx:end_idx]

            hasher = hashlib.md5()
            hasher.update(band_hashes.tobytes())
            bucket_key = f"b{band_no}_{hasher.hexdigest()[:16]}"

            lsh_indexes.append({
                "band_no": band_no,
                "bucket_key": bucket_key
            })

        return raw_signature, lsh_indexes

    #This function checks if two location strings are semantically compatible
    def _locations_compatible(self, loc_a: str, loc_b: str) -> bool:
        if not loc_a or not loc_b:
            return True 

        if loc_a == loc_b:
            return True

        if loc_a in loc_b or loc_b in loc_a:
            return True

        stopwords = {"the", "of", "in", "at", "and", "a"}
        words_a = set(loc_a.split()) - stopwords
        words_b = set(loc_b.split()) - stopwords
        return bool(words_a & words_b)

    #This function checks the similarity with the saved jobs in the database and return similar job id
    async def check_duplicate_via_backend(
        self,
        client: httpx.AsyncClient,
        backend_base_url: str,
        lsh_indexes: List[Dict[str, Any]],
        current_sig: List[int],
        auth_headers: Dict[str, str],
        incoming_location: str = "",
        jaccard_threshold: float = 0.65,
    ) -> Tuple[bool, Any]:
        
        try:
            flat_bucket_keys = [item["bucket_key"] for item in lsh_indexes]
            request_payload = {"bucket_keys": flat_bucket_keys}

            response = await client.post(f"{backend_base_url}/radar/lookup", json=request_payload, headers=auth_headers,)
            if response.status_code != 200:
                logger.warning(f"Lookup server returned unexpected status code: {response.status_code}")
                return False, None

            similar_jobs = response.json()
            if not isinstance(similar_jobs, list) or not similar_jobs:
                return False, None

            for job in similar_jobs:
                db_location = (job.get("location") or "").strip().lower()
                inc_location = incoming_location.strip().lower()

                if not self._locations_compatible(db_location, inc_location):
                    logger.info(f"Skipping Job ID {job.get('id')}: location mismatch ('{inc_location}' vs '{db_location}')")
                    continue

                meta = job.get("meta_data") or {}
                db_sig = meta.get("minhash_signature")

                if not db_sig or len(db_sig) != len(current_sig):
                    continue

                intersection = sum(1 for i, j in zip(current_sig, db_sig) if i == j)
                union = len(current_sig) + len(db_sig) - intersection
                jaccard_value = intersection / union if union > 0 else 0.0

                if jaccard_value >= jaccard_threshold:
                    matched_job_post_id = job.get("id")
                    logger.info(f"Collision detected! Jaccard score: {round(jaccard_value, 2)} against Job ID: {matched_job_post_id}")
                    return True, matched_job_post_id

        except Exception as e:
            logger.error(f"Backend radar lookup collision check failed: {e}")

        return False, None