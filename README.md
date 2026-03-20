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
python -m telemetry_server --capture
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

Open [http://localhost:8081](http://localhost:8081) in a browser to see the Race Engineer Hub UI. The panels will show "Waiting for telemetry…" until the F1 game starts sending UDP data on port 20777.

### Test data

Test `.f1bin` telemetry files are stored as GitHub release assets (too large for git). Download them with:
```shell
./scripts/download_test_data.sh
```

You can also download them manually from the GitHub releases page and place them in `data/`.

### Capturing telemetry

Add `--capture` to save all telemetry to `.f1bin` binary files in `data/`. One file is created per session, rotating automatically on session change:
```shell
python -m telemetry_server --capture
```

### Replaying telemetry

Use `--replay` to play back a captured `.f1bin` file. The web app receives the data as if it were live:
```shell
python -m telemetry_server --replay data/f1_25_capture.f1bin
```

Add `--speed` to fast-forward the replay:
```shell
python -m telemetry_server --replay data/f1_25_capture.f1bin --speed 10x
```

## F1 Race Engineer Hub Concept

<img src="race_engineer_hub.png" alt="Race Engineer Hub dashboard" width="800">

1. The top 3 panels report telemetry straight from the game.
2. Under each telemetry panel, the 'alerts' are produced by Neuro SAN
  agents constantly monitoring their telemetry data.
3. The bottom part is produced by another agent that summarizes the
  specialized agents' findings into what should be communicated to the driver.
