from abc import ABC, abstractmethod
from models.raw_job import RawJobInput


class BaseJobParser(ABC):
    @abstractmethod
    def parse_rule_based_fields(self, *args, **kwargs) -> RawJobInput:
        pass
