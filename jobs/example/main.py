import datetime
import os

log_path = os.path.join(os.path.dirname(__file__), "job.log")

with open(log_path, "a") as log_file:
    log_file.write(f"Executado em: {datetime.datetime.now().isoformat()}\n")
