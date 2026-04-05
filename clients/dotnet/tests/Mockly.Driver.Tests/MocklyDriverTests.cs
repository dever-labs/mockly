using System.Net;
using System.Net.Sockets;
using Mockly.Driver;
using Mockly.Driver.Models;
using Xunit;

namespace Mockly.Driver.Tests;

public class MocklyDriverTests
{
    [Fact]
    public void GetFreePort_ReturnsValidPort()
    {
        var listener = new TcpListener(IPAddress.Loopback, 0);
        listener.Start();
        var port = ((IPEndPoint)listener.LocalEndpoint).Port;
        listener.Stop();

        Assert.InRange(port, 1024, 65535);
    }

    [Fact]
    public void GetBinaryPath_ReturnsNull_WhenMissing()
    {
        var env = new Dictionary<string, string>(); // no MOCKLY_BINARY_PATH
        var result = MocklyInstaller.GetBinaryPath(
            Path.Combine(Path.GetTempPath(), "mockly-nonexistent-" + Guid.NewGuid()),
            env);
        Assert.Null(result);
    }

    [Fact]
    public void GetBinaryPath_FindsBinaryInBinDir()
    {
        var dir = Path.Combine(Path.GetTempPath(), "mockly-test-" + Guid.NewGuid());
        Directory.CreateDirectory(dir);
        var exeName = System.Runtime.InteropServices.RuntimeInformation.IsOSPlatform(
            System.Runtime.InteropServices.OSPlatform.Windows) ? "mockly.exe" : "mockly";
        var binaryPath = Path.Combine(dir, exeName);
        File.WriteAllText(binaryPath, "fake");

        try
        {
            var env = new Dictionary<string, string>();
            var result = MocklyInstaller.GetBinaryPath(dir, env);
            Assert.Equal(binaryPath, result);
        }
        finally
        {
            Directory.Delete(dir, true);
        }
    }

    [Fact]
    public async Task Install_Throws_WhenNoInstallSet()
    {
        var env = new Dictionary<string, string>
        {
            ["MOCKLY_NO_INSTALL"] = "1"
        };
        var opts = new InstallOptions(BinDir: Path.Combine(Path.GetTempPath(), "mockly-nope-" + Guid.NewGuid()));
        var ex = await Assert.ThrowsAsync<InvalidOperationException>(
            () => MocklyInstaller.InstallAsync(opts, env));
        Assert.Contains("MOCKLY_NO_INSTALL", ex.Message);
    }

    [Fact]
    public async Task Install_ReturnsStagedBinary_WhenPathSet()
    {
        var tmpFile = Path.Combine(Path.GetTempPath(), "fake-mockly-" + Guid.NewGuid());
        File.WriteAllText(tmpFile, "fake binary");
        try
        {
            var env = new Dictionary<string, string>
            {
                ["MOCKLY_BINARY_PATH"] = tmpFile
            };
            var result = await MocklyInstaller.InstallAsync(null, env);
            Assert.Equal(tmpFile, result);
        }
        finally
        {
            File.Delete(tmpFile);
        }
    }
}
