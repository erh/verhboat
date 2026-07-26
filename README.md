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

On startup and every 5 minutes, the component sends `?OUTLETSTAT` and
parses the device's response (e.g. `$OUTLET1 = ON`) to keep the cached
position in sync with reality — so toggles from the front panel or
BlueBOLT eventually show up via `GetPosition`.

Config:

```json
{
    "host": "192.168.1.50",
    "outlet": 1,
    "tcp-port": 23,
    "password": "secret"
}
```

- `host` — IP or hostname of the M4315-PRO (required)
- `outlet` — outlet number, 1-8 (required)
- `tcp-port` — telnet port (optional, default `23`)
- `password` — BlueBOLT-CV1 password (optional; omit if telnet auth is off)

Position `0` is off, `1` is on.

Full Viam component config example — one instance per outlet you want
to control:

```json
{
    "components": [
        {
            "name": "amp_outlet",
            "api": "rdk:component:switch",
            "model": "erh:verhboat:m4315-pro",
            "attributes": {
                "host": "192.168.1.50",
                "outlet": 1
            }
        },
        {
            "name": "subwoofer_outlet",
            "api": "rdk:component:switch",
            "model": "erh:verhboat:m4315-pro",
            "attributes": {
                "host": "192.168.1.50",
                "outlet": 2,
                "password": "secret"
            }
        }
    ]
}
```

## nicolaudie-stick3

Generic component that drives a Nicolaudie STICK-DE3 lighting controller
(for example a yacht name sign) over its UDP "quick trigger" protocol.

If a movement sensor (a GPS) is configured, the sign is turned on
automatically one hour before sunset and off at sunrise, using the sun
times computed at the sensor's current position — so it follows the boat as
it moves. Without a movement sensor there is no schedule and the sign is
controlled manually via `DoCommand`.

Config:

```json
{
    "ip": "192.168.1.60",
    "port": 2430,
    "movement_sensor": "gps",
    "color": "00FF88",
    "page": "A",
    "scene": 1
}
```

- `ip` — IP address of the STICK-DE3 (required)
- `port` — UDP quick-trigger port (optional, default `2430`)
- `movement_sensor` — name of a movement sensor / GPS used for the current
  location; enables the sunset/sunrise schedule (optional)
- `color` — hex color `RRGGBB` shown while the sign is on (optional)
- `page` — page letter (`A`, `B`, ...) or zero-based number (optional, default `A`)
- `scene` — scene number 1-50 (optional, default `1`)

`DoCommand` supports manual control and testing:

```json
{ "command": "on" }
{ "command": "off" }
{ "command": "set_color", "color": "FF0000" }
{ "command": "status" }
```

Color pattern scheduling is a planned follow-up.

# To test the yacht sign (nicolaudie-stick3)

The `cmd/yachtsign` CLI talks to the controller with the same package code
the module uses:

```
go run ./cmd/yachtsign -ip 192.168.1.60 -action on -color 00FF88
go run ./cmd/yachtsign -ip 192.168.1.60 -action off
go run ./cmd/yachtsign -ip 192.168.1.60 -action color -color FF0000
go run ./cmd/yachtsign -action suntimes -lat 40.7128 -lng -74.0060
```

# To test onehelm app
* create a directory with an index.html
* ```go run cmd/onehelm/onehelm-cmd.go -dir <directory>```
