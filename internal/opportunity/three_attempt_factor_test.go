package opportunity

import (
    "math"
    "testing"
    "propsmodel/internal/data"
)
func TestThreeAttemptFactorUsesPriorAttemptsAndConvertsRatherThanMakesAlone(t *testing.T) {
    signal := Signal{
        Stat:data.Threes, Available:true, Games:8, Through:"2026-11-18",
        Chances:48, Minutes:230, Conversion:.36,
        Rate:48.0/230.0*.36,
    }
    base:=.06
    f:=ThreeAttemptFactor(base,signal)
    if f <= 1 || f > 1.10000001 {t.Fatalf("expected capped positive attempt signal: %v",f)}
    signal.Rate=.02
    if g:=ThreeAttemptFactor(base,signal);g>=1 || g < .899999 {
        t.Fatalf("lower shot opportunities should damp 3PM, got %v",g)
    }
}
func TestThreeAttemptFactorRejectsIncompleteOrFutureLikeInsufficientEvidence(t *testing.T) {
    good:=Signal{Stat:data.Threes,Available:true,Games:8,Through:"2026-11-18",Chances:48,Minutes:230,Conversion:.37,Rate:.15}
    tests:=[]Signal{
        {Stat:data.Threes,Available:false,Games:8,Through:"2026-11-18",Chances:48,Minutes:230,Conversion:.37,Rate:.15},
        {Stat:data.Threes,Available:true,Games:2,Through:"2026-11-18",Chances:48,Minutes:230,Conversion:.37,Rate:.15},
        {Stat:data.Threes,Available:true,Games:8,Through:"2026-11-18",Chances:6,Minutes:230,Conversion:.37,Rate:.15},
        {Stat:data.Threes,Available:true,Games:8,Through:"",Chances:48,Minutes:230,Conversion:.37,Rate:.15},
        {Stat:data.Assists,Available:true,Games:8,Through:"2026-11-18",Chances:48,Minutes:230,Conversion:.37,Rate:.15},
    }
    for _,s:=range tests { if ThreeAttemptFactor(.08,s)!=1 {t.Fatalf("invalid signal changed 3PM: %+v",s)}}
    if ThreeAttemptFactor(math.NaN(),good)!=1 || ThreeAttemptFactor(0,good)!=1 {t.Fatal("invalid baseline accepted")}
}
