# windows-control-center evidence

## Commands

    cd windows && dotnet test F1Telemetry.ControlCenter.Tests
    cd windows && dotnet build F1Telemetry.ControlCenter -c Release -r win-x64

## Results

- Control-center core tests: 9 passed, 0 failed (macOS, .NET SDK 10.0.401). Covers owned-child start, unrelated-service non-adoption, matching identity adoption, executable mismatch, flush-then-stop, timeout offering retry without killing, crash exit-code preservation, voice turn cap and interruption, WAV validation.
- The WinUI project targets net10.0-windows10.0.26100.0 with Windows App SDK 1.8.260921001 and is unpackaged, self-contained, win-x64.

  Build attempts on macOS:

  - `dotnet build F1Telemetry.ControlCenter -c Release -r win-x64` -> `NETSDK1100: To build a project targeting Windows on this operating system, set the EnableWindowsTargeting property to true`.
  - with `-p:EnableWindowsTargeting=true` -> `XamlCompiler output file ... output.json was not created. The XAML compiler may have crashed.` The WinUI XAML compiler requires Windows.

  So the C# and XAML in the app project have not been compiled anywhere. Only the shared core project compiles and tests here.

## Not verified

- Windows build, portable ZIP, clean-machine launch, tray behaviour, single-instance activation, stop/flush with a real recording, microphone capture, global hotkey while F1 has focus, and playback were not executed. These need a Windows 11 x64 host with a microphone and the game.
- windows/package.ps1 is written and referenced by CI, but has not run.
