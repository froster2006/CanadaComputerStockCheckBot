
@echo off
:start
echo Starting main.py...
python main.py
echo main.py exited. Restarting in 500 seconds...
timeout /t 500 /nobreak > nul
goto start

