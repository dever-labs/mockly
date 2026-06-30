use serde::{Deserialize, Serialize};
use std::collections::HashMap;

/// A single recorded HTTP request captured by the Mockly server.
#[derive(Debug, Clone, Deserialize)]
pub struct CallEntry {
    pub id: String,
    pub timestamp: String,
    pub protocol: String,
    #[serde(default)]
    pub method: Option<String>,
    pub path: String,
    #[serde(default)]
    pub status: Option<u16>,
    pub duration_ms: i64,
    #[serde(default)]
    pub headers: HashMap<String, String>,
    #[serde(default)]
    pub body: Option<String>,
    #[serde(default)]
    pub matched_id: Option<String>,
    #[serde(default)]
    pub path_params: HashMap<String, String>,
}

/// Summary of recorded calls for a specific HTTP mock.
#[derive(Debug, Clone, Deserialize)]
pub struct CallSummary {
    pub mock_id: String,
    pub count: i64,
    pub calls: Vec<CallEntry>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MockRequest {
    pub method: String,
    pub path: String,
    #[serde(skip_serializing_if = "HashMap::is_empty", default)]
    pub headers: HashMap<String, String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MockResponse {
    pub status: u16,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub body: Option<String>,
    #[serde(skip_serializing_if = "HashMap::is_empty", default)]
    pub headers: HashMap<String, String>,
    /// Duration string, e.g. "50ms"
    #[serde(skip_serializing_if = "Option::is_none")]
    pub delay: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MockResponsePatch {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub status: Option<u16>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub body: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub headers: Option<HashMap<String, String>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub delay: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Mock {
    pub id: String,
    pub request: MockRequest,
    pub response: MockResponse,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ScenarioPatch {
    pub mock_id: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub status: Option<u16>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub body: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub headers: Option<HashMap<String, String>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub delay: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub disabled: Option<bool>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Scenario {
    pub id: String,
    pub name: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,
    pub patches: Vec<ScenarioPatch>,
}

#[derive(Debug, Clone, Deserialize)]
pub struct ActiveScenariosResponse {
    pub active: Vec<String>,
    pub scenarios: Vec<Scenario>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FaultConfig {
    pub enabled: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub delay: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub status_override: Option<u16>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error_rate: Option<f64>,
}

#[derive(Debug, Default)]
pub struct ServerOptions {
    pub scenarios: Vec<Scenario>,
}

#[derive(Debug, Default)]
pub struct InstallOptions {
    pub version: Option<String>,
    pub base_url: Option<String>,
    pub bin_dir: Option<String>,
    pub force: bool,
}
