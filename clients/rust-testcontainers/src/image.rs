use std::borrow::Cow;

use testcontainers::core::ports::ContainerPort;
use testcontainers::core::wait::HttpWaitStrategy;
use testcontainers::core::{Image, WaitFor};
use testcontainers::CopyToContainer;

pub const DEFAULT_IMAGE: &str = "ghcr.io/dever-labs/mockly";
pub const DEFAULT_TAG: &str = "latest";
pub const HTTP_PORT: u16 = 8090;
pub const API_PORT: u16 = 9091;

const CONTAINER_CONFIG_PATH: &str = "/config/mockly.yaml";
const DEFAULT_CONFIG: &str =
    "mockly:\n  api:\n    port: 9091\nprotocols:\n  http:\n    enabled: true\n    port: 8090\n";
const EXPOSED_PORTS: [ContainerPort; 2] = [ContainerPort::Tcp(HTTP_PORT), ContainerPort::Tcp(API_PORT)];

#[derive(Debug, Clone)]
pub struct MocklyImage {
    tag: String,
    config_yaml: String,
    copy_to_sources: Vec<CopyToContainer>,
}

impl Default for MocklyImage {
    fn default() -> Self {
        Self::new(DEFAULT_TAG.to_string(), DEFAULT_CONFIG.to_string())
    }
}

impl MocklyImage {
    fn new(tag: String, config_yaml: String) -> Self {
        let copy_to_sources = vec![CopyToContainer::new(
            config_yaml.as_bytes().to_vec(),
            CONTAINER_CONFIG_PATH,
        )];

        Self {
            tag,
            config_yaml,
            copy_to_sources,
        }
    }

    pub fn with_tag(self, tag: impl Into<String>) -> Self {
        Self::new(tag.into(), self.config_yaml)
    }

    pub fn with_inline_config(self, yaml: impl Into<String>) -> Self {
        Self::new(self.tag, yaml.into())
    }
}

impl Image for MocklyImage {
    fn name(&self) -> &str {
        DEFAULT_IMAGE
    }

    fn tag(&self) -> &str {
        &self.tag
    }

    fn ready_conditions(&self) -> Vec<WaitFor> {
        vec![WaitFor::Http(Box::new(
            HttpWaitStrategy::new("/api/protocols")
                .with_port(ContainerPort::Tcp(API_PORT))
                .with_expected_status_code(200u16),
        ))]
    }

    fn expose_ports(&self) -> &[ContainerPort] {
        &EXPOSED_PORTS
    }

    fn cmd(&self) -> impl IntoIterator<Item = impl Into<Cow<'_, str>>> {
        ["start", "-c", CONTAINER_CONFIG_PATH]
    }

    fn copy_to_sources(&self) -> impl IntoIterator<Item = &CopyToContainer> {
        self.copy_to_sources.iter()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn default_image_uses_latest_tag() {
        let image = MocklyImage::default();
        assert_eq!(image.name(), DEFAULT_IMAGE);
        assert_eq!(image.tag(), DEFAULT_TAG);
    }

    #[test]
    fn inline_config_overrides_config_yaml() {
        let image = MocklyImage::default().with_inline_config("mockly: {}");
        assert_eq!(image.config_yaml, "mockly: {}");
    }

    #[test]
    fn default_config_exposes_api_port() {
        let image = MocklyImage::default();
        assert!(image.config_yaml.contains("port: 9091"));
    }

    #[test]
    fn image_starts_with_config_file() {
        let image = MocklyImage::default();
        let cmd: Vec<String> = image
            .cmd()
            .into_iter()
            .map(Into::into)
            .map(|s: Cow<'_, str>| s.into_owned())
            .collect();

        assert_eq!(cmd, vec!["start", "-c", CONTAINER_CONFIG_PATH]);
        assert_eq!(image.copy_to_sources().into_iter().count(), 1);
    }
}
