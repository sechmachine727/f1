# Formula 1 racing team

## Setup

Create a dedicated Python virtual environment:

```bash
python -m venv venv
```

Source it:

* For Windows:

  ```cmd
  .\venv\Scripts\activate.bat && set PYTHONPATH=%CD%
  ```

* For Mac:

  ```bash
  source venv/bin/activate && export PYTHONPATH=`pwd`
  ```

Install the requirements:

```bash
pip install -r requirements.txt
```

## Agent network

Start a Neuro SAN race engineer agent network:
```shell
python -m run
```

## Telemetry

[EA F1 UDP specification](https://forums.ea.com/blog/f1-games-game-info-hub-en/ea-sports%E2%84%A2-f1%C2%AE25-udp-specification/12187347)

[F1 25 Telemetry Application (w/ PySide6)](https://github.com/Fredrik2002/f1-25-telemetry-application#)

Test listening to UDP packets:
```shell
python -m test_udp
```

Run a sample script to capture telemetry:
```shell
python -m lag_logger
```

## Reinforcement Learning

[Explainable Reinforcement Learning for Formula One Race Strategy](https://arxiv.org/abs/2501.04068)

## F1 Race Engineer Hub

Start the telemetry WebSocket bridge (streams live F1 25 UDP data to the web app):
```shell
python -m telemetry_server
```

In a separate terminal, start the web app:
```shell
cd race_engineer_hub
npm install
npm run dev
```

The panels will show "Waiting for telemetry…" until the F1 game starts sending UDP data on port 20777.

### Test data

Telemetry CSV files are stored as GitHub release assets (too large for git). Download them with:
```shell
./scripts/download_test_data.sh
```
This pulls all CSV files from the `test-data-v1` release into `data/`.

You can also download them manually from the [test-data-v1 release](https://github.com/cognizant-ai-lab/f1/releases/tag/test-data-v1) and place them in `data/`.

### Capturing telemetry

Add `--capture` to save all telemetry to CSV files in `data/`. One file is created per session, named after the GP and session type:
```shell
python -m telemetry_server --capture
```
Example output: `data/f1_25_bahrain_gp_race_20260214-215852.csv`

### Replaying telemetry

Use `--replay` to play back a captured CSV file. The web app receives the data as if it were live:
```shell
python -m telemetry_server --replay data/f1_25_bahrain_gp_race_20260214-215852.csv
```

Add `--speed` to fast-forward the replay:
```shell
python -m telemetry_server --replay data/f1_25_bahrain_gp_race_20260214-215852.csv --speed 10x
```

## F1 Race Engineer Hub Concept

![race_engineer_hub.png](race_engineer_hub.png)

1. The top 3 panels report telemetry straight from the game.
2. Under each telemetry panel, the 'alerts' are produced by Neuro SAN
  agents constantly monitoring their telemetry data.
3. The bottom part is produced by another agent that summarizes the
  specialized agents' findings into what should be communicated to the driver.
