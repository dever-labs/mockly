namespace Testcontainers.Mockly;

/// <summary>Fluent builder for <see cref="MocklyContainer"/>.</summary>
public sealed class MocklyBuilder : ContainerBuilder<MocklyBuilder, MocklyContainer, MocklyConfiguration>
{
    /// <summary>The default Docker image used when no custom image is specified.</summary>
    public const string DefaultImage = "ghcr.io/dever-labs/mockly:latest";

    /// <summary>The container port on which Mockly serves HTTP mocks.</summary>
    public const ushort HttpPort = 8090;

    /// <summary>The container port on which the Mockly management API listens.</summary>
    public const ushort ApiPort = 9091;

    internal const string DefaultConfigYaml = """
        mockly:
          api:
            port: 9091
        protocols:
          http:
            enabled: true
            port: 8090
        """;

    internal const string ContainerConfigPath = "/config/mockly.yaml";

    /// <summary>Initialises a new <see cref="MocklyBuilder"/> with default configuration.</summary>
    public MocklyBuilder()
        : this(new MocklyConfiguration())
    {
        DockerResourceConfiguration = Init().WithImage(DefaultImage).DockerResourceConfiguration;
    }

    private MocklyBuilder(MocklyConfiguration resourceConfiguration)
        : base(resourceConfiguration)
    {
        DockerResourceConfiguration = resourceConfiguration;
    }

    /// <inheritdoc />
    protected override MocklyConfiguration DockerResourceConfiguration { get; }

    /// <summary>
    /// Overrides the YAML configuration file copied into the container at startup.
    /// The provided YAML replaces the default config; use this to customise ports,
    /// protocols, or scenarios.
    /// </summary>
    /// <param name="yaml">Full YAML content to write to <c>/config/mockly.yaml</c>.</param>
    public MocklyBuilder WithInlineConfig(string yaml)
    {
        return Merge(DockerResourceConfiguration, new MocklyConfiguration(yaml))
            .WithResourceMapping(Encoding.UTF8.GetBytes(yaml), ContainerConfigPath);
    }

    /// <inheritdoc />
    public override MocklyContainer Build()
    {
        Validate();
        return new MocklyContainer(DockerResourceConfiguration);
    }

    /// <inheritdoc />
    protected override MocklyBuilder Init()
    {
        return base.Init()
            .WithPortBinding(HttpPort, true)
            .WithPortBinding(ApiPort, true)
            .WithCommand("start", "-c", ContainerConfigPath)
            .WithResourceMapping(Encoding.UTF8.GetBytes(DefaultConfigYaml), ContainerConfigPath)
            .WithWaitStrategy(Wait.ForUnixContainer().UntilHttpRequestIsSucceeded(request => request.ForPort(ApiPort).ForPath("/api/protocols")));
    }

    /// <inheritdoc />
    protected override MocklyBuilder Clone(IResourceConfiguration<CreateContainerParameters> resourceConfiguration)
    {
        return Merge(DockerResourceConfiguration, new MocklyConfiguration(resourceConfiguration));
    }

    /// <inheritdoc />
    protected override MocklyBuilder Clone(IContainerConfiguration resourceConfiguration)
    {
        return Merge(DockerResourceConfiguration, new MocklyConfiguration(resourceConfiguration));
    }

    /// <inheritdoc />
    protected override MocklyBuilder Merge(MocklyConfiguration oldValue, MocklyConfiguration newValue)
    {
        return new MocklyBuilder(new MocklyConfiguration(oldValue, newValue));
    }
}
