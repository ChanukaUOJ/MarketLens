# Why this is used inside only the research folder? not in the crawler?

JOB_EXTRACTION_SCHEMA = {
    "type": "object",
    "properties": {
        "employer": {
            "type": "object",
            "properties": {
                "name": {"type": "string"}
            },
            "required": ["name"]
        },
        "job_role": {"type": "string"},
        "job_type": {
            "type": "object",
            "properties": {
                "type": {"type": "string"}
            },
            "required": ["type"]
        },
        "job_description": {"type": "string"},
        "location": {"type": "string"},
        "is_remote": {"type": "boolean"},
        "meta_data": {
            "type": "object",
            "properties": {
                "geo_data": {
                    "type": "object",
                    "properties": {
                        "province": {"type": "string"}
                    },
                    "required": ["province"]
                },
                "industry": {
                    "type": "object",
                    "properties": {
                        "name": {"type": "string"}
                    },
                    "required": ["name"]
                },
                "occupation": {
                    "type": "object",
                    "properties": {
                        "name": {"type": "string"}
                    },
                    "required": ["name"]
                },
                "education_level": {
                    "type": "object",
                    "properties": {
                        "level": {"type": "string"}
                    },
                    "required": ["level"]
                },
                "experience": {
                    "type": "object",
                    "properties": {
                        "name": {"type": "string"}
                    },
                    "required": ["name"]
                },
                "source": {
                    "type": "object",
                    "properties": {
                        "source": {"type": "string"}
                    },
                    "required": ["source"]
                },
                "ai_version": {
                    "type": "object",
                    "properties": {
                        "version": {"type": "string"}
                    },
                    "required": ["version"]
                },
                "posted_at": {
                    "type": "string",
                    "pattern": r"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$"
                },
                "confidence_score": {"type": "number"},
            },
            "required": [
                "geo_data", "industry", "occupation",
                "education_level", "experience", "source", "ai_version",
                "posted_at", "confidence_score"
            ]
        },
        "skills": {
            "type": "array",
            "items": {
                "type": "object",
                "properties": {
                    "skill": {"type": "string"}
                },
                "required": ["skill"]
            }
        }
    },
    "required": [
        "employer", "job_role", "job_type", "job_description",
        "location", "is_remote", "meta_data", "skills"
    ]
}


BASE_JOB_INSTRUCTION = """
Extract structural data from the raw job text into the specified JSON format.
CRITICAL: If a field is missing, use the strict default provided. Do not hallucinate.

Fields & Defaults:

1. 'employer': Object with company name.
   Format: {"name": "Company Name"}
   Default: {"name": ""}

2. 'job_role': Vacancy title string.
   Default: ""

3. 'job_type': Object with contract type.
   Format: {"type": "Full Time"} or {"type": "Part Time"} or {"type": "Contract"} or {"type": "Internship"}
   Default: {"type": "Full Time"}

4. 'job_description': Combined summary of responsibilities, qualifications, and any offered benefits. Single string.
   Default: ""

5. 'location': City/region string (e.g. "Colombo 03, Sri Lanka").
   Default: "Sri Lanka"

6. 'is_remote': true ONLY if explicit remote wording exists, else false.

7. 'meta_data': Object containing all metadata fields:

   - 'geo_data': Infer the Sri Lankan province from the location text.
     Format: {"province": "<province>"}
     Must match EXACTLY ONE from this list:
       ["Western", "Central", "Northern", "Eastern", "North Western",
        "North Central", "Uva", "Southern", "Sabaragamuwa"]
     Default: {"province": "Western"}

  - 'industry': Match the job to EXACTLY ONE industry from this list:
     Format: {"name": "<industry>"}
     Must match EXACTLY ONE from this list:
       ["Accommodation and Food Service Activities",
        "Other Service Activities",
        "Manufacturing",
        "Wholesale and Retail Trade; Repair of Motor Vehicles and Motorcycles",
        "Construction",
        "Activities of Households as Employers",
        "Administrative and Support Service Activities",
        "Agriculture, Forestry and Fishing",
        "Transportation and Storage",
        "Human Health and Social Work Activities",
        "Education",
        "Professional, Scientific and Technical Activities",
        "Financial and Insurance Activities",
        "Information and Communication",
        "Public administration and Defence; Compulsory Social Security",
        "Arts, Entertainment and Recreation",
        "Electricity, Gas, Steam and Air Conditioning Supply",
        "Real Estate Activities",
        "Water Supply; Sewerage, Waste Management and Remediation Activities",
        "Mining and Quarrying",
        "Activities of Extraterritorial Organizations and Bodies"]
     Default: {"name": "Other Service Activities"}

   - 'occupation': Match the job role to EXACTLY ONE occupation from this list:
     Format: {"name": "<occupation>"}
     Must match EXACTLY ONE from this list:
       ["Elementary Occupations",
        "Service Workers & Shop & Market Sales Workers",
        "Craft & Related Workers",
        "Plant and Machine Operators and Assemblers",
        "Technicians & Associate professionals",
        "Clerks",
        "Legislators, Senior Officials And Managers",
        "Professionals",
        "Skilled Agricultural & Fishery Workers",
        "Armed Forces"]
     Default: {"name": "Professionals"}

   - 'education_level': Minimum education required for the role.
     Format: {"level": "<level>"}
     Must match EXACTLY ONE from this list:
       ["Degree & Above",
        "GCE A/L",
        "GCE O/L",
        "Below GCE O/L",
        "Not Specified"]
     Default: {"level": "Not Specified"}

   - 'experience': Whether the role requires prior experience.
     Format: {"name": "<value>"}
     Must match EXACTLY ONE from this list:
       ["Experience Required", "Not Specified"]
     Default: {"name": "Not Specified"}

   - 'source': The website or platform this job was crawled from.
     Format: {"source": "<source name>"}
     Example: {"source": "Ikman"} or {"source": "TopJobs"}
     Default: {"source": "Unknown"}

   - 'ai_version': The AI model version used for extraction.
     Format: {"version": "deepseek-v1"}
     Always use: {"version": "deepseek-v1"}

   - 'posted_at': ISO 8601 timestamp of when the job was posted, if detectable from the page.
     Format: "YYYY-MM-DDTHH:MM:SSZ"
     Default: current UTC timestamp.

   - 'confidence_score': Your confidence in the extraction accuracy.
     Float between 0.00 and 1.00. Use 1.00 if all fields are clearly present.
     Default: 0.80

8. 'skills': Array of skill objects extracted from the job description.
   Format: [{"skill": "Skill Name"}, ...]
   Extract only concrete technical or professional skills mentioned.
   Default: []
"""