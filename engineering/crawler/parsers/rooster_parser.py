from parsers.base_parser import BaseJobParser

class RoosterParser(BaseJobParser):

    def parse_rule_based_fields(self, job: dict) -> dict:
        # instead of parsing this as a dict can we wrap this with a pydantic model and parse for the type safe?
        return {
            "employer": job.get("company_name"),
            "job_role": job.get("title"),
            "location": job.get("location"),
            "description": job.get("description")
        }