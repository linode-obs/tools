## Usage: python3 stress.py <msgpersec> [processes]
## Process argument defaults to 3 if not specified

import os
import signal
import subprocess
import sys
import time

# EDIT THESE
LOG_DIR = "/path/to/logs"
LOG_STRESSOR = "/path/to/log-stressor/binary"
# END EDITING
SIZE_LIMIT = 1 * 1024 * 1024 * 1024  # 1GB in bytes; max size before truncation
TRUNCATE_SIZE = 50 * 1024 * 1024  # 50MB in bytes; size to truncate to
PROCESSES = []


# Kill all log-stressor processes
# When using this, we ran it in screen/tmux and `CTRL+C` was leaving around the `log-stressor` processes.
def cleanup(signum, frame):
    os.system(f"pkill -f '{LOG_STRESSOR} -file {LOG_DIR}'")
    sys.exit(0)


# truncate cause disk space isn't free, don't want to fill the disk in a stress test
# you can modify `TRUNCATE_SIZE` to whatever you want if this doesn't matter to you
def truncate_log_file(log_file):
    with open(log_file, "rb") as f:
        f.seek(-TRUNCATE_SIZE, os.SEEK_END)
        data = f.read()
    with open(log_file, "wb") as f:
        f.write(data)


# start each process with the file it's supposed to write to
def start_log_stressor(log_file, msgpersec):
    return subprocess.Popen(
        [LOG_STRESSOR, "-file", log_file, "-msgpersec", str(msgpersec)]
    )


# take args, start process(es)
def main(msgpersec, num_processes):
    global PROCESSES
    LOG_FILES = [
        os.path.join(LOG_DIR, f"log-stressor{i}.log") for i in range(num_processes)
    ]

    # Ensure log files exist
    for log_file in LOG_FILES:
        # create'em if they don't
        if not os.path.exists(log_file):
            open(log_file, "w").close()

    PROCESSES = [start_log_stressor(log_file, msgpersec) for log_file in LOG_FILES]

    try:
        while True:
            for i, log_file in enumerate(LOG_FILES):
                # if our log file fills up, truncate
                if os.path.getsize(log_file) > SIZE_LIMIT:
                    process = PROCESSES[i]
                    if process and process.poll() is None:
                        process.kill()  # Forcefully kill the process
                        process.wait()
                    # cut it down to `TRUNCATE_SIZE` and restart the process
                    truncate_log_file(log_file)
                    PROCESSES[i] = start_log_stressor(log_file, msgpersec)

            time.sleep(1)
    except KeyboardInterrupt:
        cleanup(None, None)


if __name__ == "__main__":
    if len(sys.argv) not in [2, 3]:
        print(f"Usage: {sys.argv[0]} <msgpersec> [<num_processes>]")
        sys.exit(1)

    msgpersec = int(sys.argv[1])
    num_processes = int(sys.argv[2]) if len(sys.argv) == 3 else 3

    # Register the cleanup function to handle SIGINT (Ctrl + C)
    signal.signal(signal.SIGINT, cleanup)
    main(msgpersec, num_processes)
