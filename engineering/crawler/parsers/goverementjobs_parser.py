from parsers.base_parser import BaseJobParser

class GoverementJobsParser(BaseJobParser):

    def parse_rule_based_fields(self, data: dict) -> dict:
        # instead of parsing this as a dict can we wrap this with a pydantic model and parse for the type safe?
        return {
            "employer": data.get("employer", "N/A"),
            "job_role": data.get("title", "N/A"),
            "location": data.get("location", "N/A"),
            "description": data.get("description", "")
        }