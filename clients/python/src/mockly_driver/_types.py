from dataclasses import dataclass, field
from typing import Optional


@dataclass
class MockRequest:
    method: str
    path: str
    headers: dict[str, str] = field(default_factory=dict)


@dataclass
class MockResponse:
    status: int
    body: str = ""
    headers: dict[str, str] = field(default_factory=dict)
    delay: Optional[str] = None  # e.g. "50ms"


@dataclass
class Mock:
    id: str
    request: MockRequest
    response: MockResponse


@dataclass
class ScenarioPatch:
    mock_id: str
    status: Optional[int] = None
    body: Optional[str] = None
    delay: Optional[str] = None


@dataclass
class Scenario:
    id: str
    name: str
    patches: list[ScenarioPatch]


@dataclass
class FaultConfig:
    enabled: bool
    delay: Optional[str] = None        # e.g. "200ms"
    status_override: Optional[int] = None
    error_rate: Optional[float] = None  # 0.0–1.0
