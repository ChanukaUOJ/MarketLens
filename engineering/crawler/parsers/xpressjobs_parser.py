from parsers.base_parser import BaseJobParser

class XpressJobsParser(BaseJobParser):

    def parse_rule_based_fields(self, job: dict) -> dict:
        # instead of parsing this as a dict can we wrap this with a pydantic model and parse for the type safe?
        return {
            "employer": job.get("employer"),
            "job_role": job.get("job_title"),
            "location": job.get("location"),
            "description": job.get("description")
        }