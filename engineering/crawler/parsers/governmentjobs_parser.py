from parsers.base_parser import BaseJobParser
from models.raw_job import RawJobInput


class GovernmentJobsParser(BaseJobParser):
    def parse_rule_based_fields(self, data: dict, crawler_run_id: int) -> RawJobInput:
        return RawJobInput(
            employer=data.get("employer", ""),
            job_role=data.get("title", ""),
            location=data.get("location", ""),
            description=data.get("description", ""),
            crawler_run_id=crawler_run_id,
            source="GovernmentJobs",
        )
