namespace Testcontainers.Mockly;

/// <summary>Holds Mockly-specific configuration merged by <see cref="MocklyBuilder"/>.</summary>
public sealed class MocklyConfiguration : ContainerConfiguration
{
    /// <summary>Initialises a new configuration with an optional inline YAML override.</summary>
    public MocklyConfiguration(string? inlineConfig = null)
    {
        InlineConfig = inlineConfig;
    }

    /// <inheritdoc />
    public MocklyConfiguration(IResourceConfiguration<CreateContainerParameters> resourceConfiguration)
        : base(resourceConfiguration)
    {
    }

    /// <inheritdoc />
    public MocklyConfiguration(IContainerConfiguration resourceConfiguration)
        : base(resourceConfiguration)
    {
    }

    /// <inheritdoc />
    public MocklyConfiguration(MocklyConfiguration resourceConfiguration)
        : this(new MocklyConfiguration(), resourceConfiguration)
    {
    }

    /// <inheritdoc />
    public MocklyConfiguration(MocklyConfiguration oldValue, MocklyConfiguration newValue)
        : base(oldValue, newValue)
    {
        InlineConfig = BuildConfiguration.Combine(oldValue.InlineConfig, newValue.InlineConfig);
    }

    /// <summary>
    /// The raw YAML string set via <see cref="MocklyBuilder.WithInlineConfig"/>,
    /// or <see langword="null"/> when using the default configuration.
    /// </summary>
    public string? InlineConfig { get; }
}
