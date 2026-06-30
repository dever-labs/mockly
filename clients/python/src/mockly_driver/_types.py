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
class MockResponsePatch:
    status: Optional[int] = None
    body: Optional[str] = None
    headers: Optional[dict[str, str]] = None
    delay: Optional[str] = None


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
    headers: Optional[dict[str, str]] = None
    delay: Optional[str] = None
    disabled: Optional[bool] = None


@dataclass
class Scenario:
    id: str
    name: str
    patches: list[ScenarioPatch]
    description: Optional[str] = None


@dataclass
class ActiveScenariosResponse:
    active: list[str] = field(default_factory=list)
    scenarios: list[Scenario] = field(default_factory=list)


@dataclass
class CallEntry:
    id: str
    timestamp: str
    protocol: str
    path: str
    duration_ms: int
    method: Optional[str] = None
    status: Optional[int] = None
    headers: dict[str, str] = field(default_factory=dict)
    body: Optional[str] = None
    matched_id: Optional[str] = None
    path_params: dict[str, str] = field(default_factory=dict)


@dataclass
class CallSummary:
    mock_id: str
    count: int
    calls: list["CallEntry"] = field(default_factory=list)


@dataclass
class FaultConfig:
    enabled: bool
    delay: Optional[str] = None        # e.g. "200ms"
    status: Optional[int] = None
    error_rate: Optional[float] = None  # 0.0–1.0
