using System.ComponentModel;
using System.Diagnostics;
using System.Runtime.InteropServices;
using Microsoft.Win32.SafeHandles;

namespace FlowLight.Core;

// Windows closes the job handle if the service crashes. All owned network
// processes then terminate instead of surviving with stale routes and keys.
internal sealed class OwnedProcessJob : IDisposable
{
    readonly SafeFileHandle handle;
    public OwnedProcessJob()
    {
        handle = CreateJobObject(IntPtr.Zero, null);
        if (handle.IsInvalid) throw new Win32Exception(Marshal.GetLastWin32Error());
        var info = new ExtendedLimits { Basic = new BasicLimits { LimitFlags = 0x2000 } };
        int length = Marshal.SizeOf<ExtendedLimits>(); var ptr = Marshal.AllocHGlobal(length);
        try { Marshal.StructureToPtr(info, ptr, false); if (!SetInformationJobObject(handle, 9, ptr, (uint)length)) throw new Win32Exception(Marshal.GetLastWin32Error()); }
        catch { handle.Dispose(); throw; }
        finally { Marshal.FreeHGlobal(ptr); }
    }
    public void Add(Process process) { if (!AssignProcessToJobObject(handle, process.SafeHandle)) throw new Win32Exception(Marshal.GetLastWin32Error()); }
    public void Dispose() => handle.Dispose();
    [StructLayout(LayoutKind.Sequential)] struct BasicLimits { public long ProcessTime, JobTime; public uint LimitFlags; public UIntPtr MinWorkingSet, MaxWorkingSet; public uint ActiveProcessLimit; public UIntPtr Affinity; public uint Priority, Scheduling; }
    [StructLayout(LayoutKind.Sequential)] struct IOCounters { public ulong ReadOperations, WriteOperations, OtherOperations, ReadBytes, WriteBytes, OtherBytes; }
    [StructLayout(LayoutKind.Sequential)] struct ExtendedLimits { public BasicLimits Basic; public IOCounters IO; public UIntPtr ProcessMemory, JobMemory, PeakProcessMemory, PeakJobMemory; }
    [DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)] static extern SafeFileHandle CreateJobObject(IntPtr attributes, string? name);
    [DllImport("kernel32.dll", SetLastError = true)] static extern bool SetInformationJobObject(SafeFileHandle job, int infoClass, IntPtr info, uint length);
    [DllImport("kernel32.dll", SetLastError = true)] static extern bool AssignProcessToJobObject(SafeFileHandle job, SafeProcessHandle process);
}
