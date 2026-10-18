defmodule Mockly.Testcontainers.MixProject do
  use Mix.Project

  # SPIKE (#371): version is intentionally NOT wired into release-please yet.
  # See README.md for status/scope before this gets a real release train.
  @version "0.1.0"
  @source_url "https://github.com/dever-labs/mockly"

  def project do
    [
      app: :mockly_testcontainers,
      version: @version,
      elixir: "~> 1.14",
      start_permanent: Mix.env() == :prod,
      elixirc_paths: elixirc_paths(Mix.env()),
      deps: deps(),
      description: description(),
      package: package(),
      name: "Mockly.Testcontainers"
    ]
  end

  defp elixirc_paths(:test), do: ["lib", "test/support"]
  defp elixirc_paths(_), do: ["lib"]

  def application do
    [
      extra_applications: [:logger, :inets, :ssl]
    ]
  end

  defp deps do
    [
      {:testcontainers, "~> 2.4"},
      {:jason, "~> 1.4"}
    ]
  end

  defp description do
    "Testcontainers module for Mockly — run Mockly as a Docker container in ExUnit tests (SPIKE, see #371)"
  end

  defp package do
    [
      licenses: ["MIT"],
      links: %{"GitHub" => @source_url},
      files: ~w(lib mix.exs README.md)
    ]
  end
end
