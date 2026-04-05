using System.Runtime.InteropServices;
using Mockly.Driver.Models;

namespace Mockly.Driver;

public static class MocklyInstaller
{
    public const string DefaultVersion = "v0.1.0";

    public static string? GetBinaryPath(string? binDir = null)
        => GetBinaryPath(binDir, null);

    internal static string? GetBinaryPath(string? binDir, IReadOnlyDictionary<string, string>? env)
    {
        var envPath = GetEnv("MOCKLY_BINARY_PATH", env);
        if (!string.IsNullOrEmpty(envPath) && File.Exists(envPath))
            return envPath;

        var exeName = BinaryName();
        var dirs = new List<string?> { binDir, Path.Combine(AppContext.BaseDirectory, "bin") };
        foreach (var dir in dirs)
        {
            if (dir == null) continue;
            var candidate = Path.Combine(dir, exeName);
            if (File.Exists(candidate)) return candidate;
        }
        return null;
    }

    public static Task<string> InstallAsync(InstallOptions? opts = null)
        => InstallAsync(opts, null);

    internal static async Task<string> InstallAsync(InstallOptions? opts, IReadOnlyDictionary<string, string>? env)
    {
        opts ??= new InstallOptions();

        var envPath = GetEnv("MOCKLY_BINARY_PATH", env);
        if (!string.IsNullOrEmpty(envPath) && File.Exists(envPath))
            return envPath;

        var existing = GetBinaryPath(opts.BinDir, env);
        if (existing != null && !opts.Force)
            return existing;

        if (!string.IsNullOrEmpty(GetEnv("MOCKLY_NO_INSTALL", env)))
            throw new InvalidOperationException(
                "MOCKLY_NO_INSTALL is set; refusing to download Mockly binary.");

        var version = opts.Version
            ?? GetEnv("MOCKLY_VERSION", env)
            ?? DefaultVersion;

        var baseUrl = opts.BaseUrl
            ?? GetEnv("MOCKLY_DOWNLOAD_BASE_URL", env)
            ?? "https://github.com/dever-labs/mockly/releases/download";

        var asset = GetAssetName();
        var url = $"{baseUrl.TrimEnd('/')}/{version}/{asset}";

        var binDir = opts.BinDir ?? Path.Combine(AppContext.BaseDirectory, "bin");
        Directory.CreateDirectory(binDir);

        var dest = Path.Combine(binDir, BinaryName());
        await DownloadFileAsync(url, dest);

        if (!RuntimeInformation.IsOSPlatform(OSPlatform.Windows))
        {
            var chmod = new System.Diagnostics.ProcessStartInfo("chmod", $"+x \"{dest}\"")
            {
                UseShellExecute = false,
                RedirectStandardError = true,
            };
            using var proc = System.Diagnostics.Process.Start(chmod)!;
            await proc.WaitForExitAsync();
        }

        return dest;
    }

    private static string GetAssetName()
    {
        var os = RuntimeInformation.IsOSPlatform(OSPlatform.Windows) ? "windows"
                : RuntimeInformation.IsOSPlatform(OSPlatform.OSX) ? "darwin"
                : "linux";

        var arch = RuntimeInformation.OSArchitecture switch
        {
            Architecture.X64 => "amd64",
            Architecture.Arm64 => "arm64",
            _ => throw new PlatformNotSupportedException($"Unsupported architecture: {RuntimeInformation.OSArchitecture}")
        };

        var ext = RuntimeInformation.IsOSPlatform(OSPlatform.Windows) ? ".exe" : "";
        return $"mockly-{os}-{arch}{ext}";
    }

    private static string BinaryName()
        => RuntimeInformation.IsOSPlatform(OSPlatform.Windows) ? "mockly.exe" : "mockly";

    private static async Task DownloadFileAsync(string url, string dest)
    {
        using var client = new HttpClient(new HttpClientHandler { AllowAutoRedirect = true });
        client.DefaultRequestHeaders.Add("User-Agent", "Mockly.Driver/0.1.0");
        using var response = await client.GetAsync(url, HttpCompletionOption.ResponseHeadersRead);
        response.EnsureSuccessStatusCode();
        using var fs = new FileStream(dest, FileMode.Create, FileAccess.Write, FileShare.None);
        await response.Content.CopyToAsync(fs);
    }

    private static string? GetEnv(string key, IReadOnlyDictionary<string, string>? env)
        => env != null
            ? (env.TryGetValue(key, out var v) ? v : null)
            : Environment.GetEnvironmentVariable(key);
}

public record InstallOptions(
    string? Version = null,
    string? BaseUrl = null,
    string? BinDir = null,
    bool Force = false);
