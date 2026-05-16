# Module verhboat 

Provide a description of the purpose of the module and any relevant information.

## alerts

```json
{
    "freshwater_tank" : <...>,
    "freshwater_spotzero" : <....>
    "alert_level" : <99>
}
```

## fw fill

```json
{
    "freshwater_tank" : <...>,
    "freshwater_spotzero" : <....>
    "freshwater_valve" : <...:

	"start_level" : 93,
	"end_level" : 98
}
```

## combined-tank

Aggregates readings from multiple tank sensors into a single sensor. All
referenced tanks must share the same `Type`; the combined sensor sums
`Capacity` and `Liters` across them and recomputes `Level` as
`(Liters / Capacity) * 100`.

```json
{
    "tanks" : ["tank_a", "tank_b", "..."]
}
```

Each entry in `tanks` is the name of another sensor whose `Readings` return
`raw`, `Capacity`, `Liters`, and `Type`. At least one tank is required.

Readings:

- `raw` — sum of `raw` across all tanks
- `Capacity` — sum of `Capacity` across all tanks
- `Liters` — sum of `Liters` across all tanks
- `Level` — combined fill percentage (0 if total capacity is 0)
- `Type` — the shared tank type

## m4315-pro

Toggle switch for one outlet on a Panamax/Furman M4315-PRO power
conditioner. Each instance controls a single outlet over the device's
local telnet interface (`!SWITCH <outlet> <ON|OFF>`).

```json
{
    "host": "192.168.1.50",
    "outlet": 1,
    "tcp-port": 23,
    "password": "..."
}
```

- `host` — IP or hostname of the M4315-PRO (required)
- `outlet` — outlet number, 1-8 (required)
- `tcp-port` — telnet port (optional, default 23)
- `password` — BlueBOLT-CV1 password (optional; omit if telnet auth is off)

Position `0` is off, `1` is on.

# To test onehelm app
* create a directory with an index.html
* ```go run cmd/onehelm/onehelm-cmd.go -dir <directory>```
