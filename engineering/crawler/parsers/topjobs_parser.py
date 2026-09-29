from parsers.base_parser import BaseJobParser


class TopJobsParser(BaseJobParser):
    def parse_rule_based_fields(self, data: dict) -> dict:
        return {
            "employer": data.get("employer", "N/A"),
            "job_role": data.get("title", "N/A"),
            "location": data.get("location", "N/A"),
            "description": data.get("ocr_text", ""),
        }
