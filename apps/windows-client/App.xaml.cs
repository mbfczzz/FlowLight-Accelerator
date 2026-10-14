using System.Threading;
using System.Windows;

namespace FlowLight.Client;

public partial class App : System.Windows.Application
{
    // One client per Windows logon session: the window closes to the tray, so
    // relaunching from the shortcut is how users get it back. A second launch
    // wakes the first instance instead of opening a second window and tray
    // icon that would both drive the same service session.
    const string InstanceName = @"Local\FlowLight.Accelerator.Client";
    const string ActivateName = InstanceName + ".Activate";

    Mutex? instance;
    bool owner;
    EventWaitHandle? activate;

    protected override void OnStartup(StartupEventArgs e)
    {
        base.OnStartup(e);
        instance = new Mutex(true, InstanceName, out owner);
        if (!owner)
        {
            try
            {
                using var existing = EventWaitHandle.OpenExisting(ActivateName);
                existing.Set();
            }
            catch (Exception ex) when (ex is WaitHandleCannotBeOpenedException or UnauthorizedAccessException)
            {
                // The first instance is still starting or is being torn down;
                // the user can simply launch again.
            }
            Shutdown();
            return;
        }

        activate = new EventWaitHandle(false, EventResetMode.AutoReset, ActivateName);
        var window = new MainWindow();
        MainWindow = window;
        window.Show();

        var handle = activate;
        var waiter = new Thread(() =>
        {
            while (true)
            {
                try
                {
                    handle.WaitOne();
                }
                catch (ObjectDisposedException)
                {
                    return;
                }
                Dispatcher.InvokeAsync(window.ShowFromTray);
            }
        })
        {
            IsBackground = true,
            Name = "activate-wait",
        };
        waiter.Start();
    }

    protected override void OnExit(ExitEventArgs e)
    {
        activate?.Dispose();
        if (owner)
        {
            instance?.ReleaseMutex();
        }
        instance?.Dispose();
        base.OnExit(e);
    }
}
