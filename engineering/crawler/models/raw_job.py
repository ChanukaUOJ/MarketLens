import uuid

from pydantic import BaseModel, Field


class RawJobInput(BaseModel):
    job_id: str = Field(default_factory=lambda: str(uuid.uuid4()))
    employer: str = Field(default="")
    job_role: str = Field(default="")
    location: str = Field(default="")
    description: str = Field(default="")
    crawler_run_id: int = Field(gt=0)
    source: str = Field(pattern=r"\S")
