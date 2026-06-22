from dataclasses import dataclass
from typing import Optional


@dataclass
class MockRequest:
    method: str
    path: str
    headers: Optional[dict] = None


@dataclass
class MockResponse:
    status: int
    body: Optional[str] = None
    headers: Optional[dict] = None
    delay: Optional[str] = None


@dataclass
class Mock:
    id: str
    request: MockRequest
    response: MockResponse


@dataclass
class FaultConfig:
    enabled: bool
    delay: Optional[str] = None
    status_override: Optional[int] = None
    error_rate: Optional[float] = None
