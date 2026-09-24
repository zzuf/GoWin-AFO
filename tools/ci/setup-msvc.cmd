@echo off
rem Run with CALL so vcvarsall's environment remains available to the compiler.
set "ABI_VSWHERE=%ProgramFiles(x86)%\Microsoft Visual Studio\Installer\vswhere.exe"
if not exist "%ABI_VSWHERE%" (
  echo Visual Studio locator not found: "%ABI_VSWHERE%" 1>&2
  exit /b 1
)

set "ABI_VCVARS="
for /f "usebackq delims=" %%I in (`"%ABI_VSWHERE%" -latest -products * -requires Microsoft.VisualStudio.Component.VC.Tools.x86.x64 -find VC\Auxiliary\Build\vcvarsall.bat`) do set "ABI_VCVARS=%%I"
if not defined ABI_VCVARS (
  echo No Visual Studio instance with MSVC x86/x64 tools was found. 1>&2
  exit /b 1
)

call "%ABI_VCVARS%" %*
if errorlevel 1 exit /b %ERRORLEVEL%
if not defined WindowsSdkDir (
  echo vcvarsall did not set WindowsSdkDir. 1>&2
  exit /b 1
)
if /i not "%WindowsSDKVersion%"=="%~2\" (
  echo vcvarsall selected Windows SDK "%WindowsSDKVersion%" instead of "%~2". 1>&2
  exit /b 1
)
if not exist "%WindowsSdkDir%Include\%~2\um\Windows.h" (
  echo Windows SDK "%~2" headers were not found in "%WindowsSdkDir%". 1>&2
  exit /b 1
)
exit /b 0
