@echo off

set /p UserInput= "Spicetify will be Installed. If you don't agree, use the BlockTheSpot script, else press 'y' to continue (y/n)? "
if /i "%UserInput%"=="y" (
    powershell -Command "& {iwr -useb https://raw.githubusercontent.com/spicetify/spicetify-cli/master/install.ps1 | iex}"
    powershell -Command "& {iwr -useb https://raw.githubusercontent.com/spicetify/spicetify-marketplace/main/resources/install.ps1 | iex}"
    powershell -Command "& {[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12; Invoke-WebRequest -UseBasicParsing 'https://raw.githubusercontent.com/ZuanCrisp/BlockTheSpot/master/install.ps1' | Invoke-Expression}"
    pause
) else (
    echo "Patch not installed."
    pause
    exit /b
)
