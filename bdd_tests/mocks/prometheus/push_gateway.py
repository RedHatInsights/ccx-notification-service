from fastapi import FastAPI


app = FastAPI()


@app.get("/metrics")
def get_metrics():
    return {"metrics": "ok"}


@app.put("/metrics/job/ccx_notification_service")
def upload_metrics():
    return {"put_metrics": "ok"}
