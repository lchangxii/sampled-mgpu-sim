package profiler
import (
    "time"
)


type WallTime struct {
    starttimes map[string] time.Time
}
var Walltime * WallTime
func InitWallTime() {
    Walltime = &WallTime{
    }
    Walltime.starttimes = make(map[string]time.Time)
}

func ( walltime *WallTime)InitStartWallTime( flag string ) {
    _,found := walltime.starttimes[flag]
    if found {
        panic("one flag can only have one walltime")
    }
    startTime := time.Now()
    walltime.starttimes[ flag ] = startTime
}
func (walltime *WallTime)GetIntervalWallTime(flag string  ) float64{
    startTime,found := walltime.starttimes[flag]
    if !found {
        panic("init walltime should be called before")
    }
    endTime := time.Now()
    duration := endTime.Sub(startTime)
    executetime := duration.Seconds()
    delete( walltime.starttimes, flag)
    return executetime
}

