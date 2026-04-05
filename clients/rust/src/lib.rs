pub mod install;
pub mod server;
mod types;
mod utils;

pub use install::{get_binary_path, install, DEFAULT_VERSION};
pub use server::MocklyServer;
pub use types::{
    FaultConfig, InstallOptions, Mock, MockRequest, MockResponse, Scenario, ScenarioPatch,
    ServerOptions,
};
pub use utils::get_free_port;
