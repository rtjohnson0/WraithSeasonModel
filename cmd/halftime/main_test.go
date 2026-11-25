package main
import(
 "encoding/json"
 "testing"
 "time"
 "propsmodel/internal/picklog"
)
func TestHalftimeThresholdUsesStrictOver(t *testing.T){
 for _,tt:=range []struct{line,current float64;want int}{{6.5,4,3},{6.5,7,0},{7,4,4},{2.5,2,1},{0.5,0,1}}{
  if got:=thresholdRemaining(tt.line,tt.current);got!=tt.want{t.Fatalf("%v with %v: got %d want %d",tt.line,tt.current,got,tt.want)}
 }
}
func TestLiveMarketRejectsOldQuotesAfterTip(t *testing.T){
 now,_:=time.Parse(time.RFC3339,"2026-11-25T21:00:00Z")
 tip,_:=time.Parse(time.RFC3339,"2026-11-25T19:00:00Z")
 p:=func(at string)[]byte{
  data:=map[string]interface{}{"data":[]interface{}{map[string]interface{}{
   "game_id":123,"player_id":44,"vendor":"test-book",
   "prop_type":"assists","line_value":6.5,"updated_at":at,
   "market":map[string]interface{}{"type":"over_under","over_odds":-125,"under_odds":105},
  }}}
  b,_:=json.Marshal(data);return b
 }
 if len(parseLiveOffers(p("2026-11-25T18:59:00Z"),123,now,tip))!=0{t.Fatal("pregame book cannot be marked live")}
 if len(parseLiveOffers(p("2026-11-25T20:45:00Z"),123,now,tip))!=0{t.Fatal("stale quote cannot be marked live")}
 // A refreshed, paired actual in-play book quote is allowed if the feed supplies timestamps.
 got:=parseLiveOffers(p("2026-11-25T20:59:00Z"),123,now,tip)
 if len(got)!=1{t.Fatalf("valid live quote excluded: %+v",got)}
}
func TestCurrentBoxRejectsPeriodSpecificRows(t *testing.T){
 for _,tt:=range []struct{payload string; valid bool}{
  {`{"game":{"id":1},"player":{"id":2},"period":1,"pts":12,"reb":4,"ast":4,"fg3m":1}`,false},
  {`{"game":{"id":1},"player":{"id":2},"pts":12,"reb":4,"ast":4,"fg3m":1}`,true},
  {`{"game":{"id":1},"player":{"id":2},"pts":12,"reb":4,"ast":4}`,false},
 }{_,ok:=parseStats(json.RawMessage(tt.payload));if ok!=tt.valid{t.Fatal("wrong box validation",tt)}}
}
func TestMissingStatsMustNeverDisplayZero(t *testing.T){
 now,_:=time.Parse(time.RFC3339,"2026-11-25T20:00:00Z")
 entry:=picklog.Entry{Date:"2026-11-25",GameID:11,PlayerID:22,Player:"Test",Stat:"assists",Side:"OVER",Line:6.5,Tip:"2026-11-25T19:00:00Z",LoggedAt:"2026-11-25T18:10:00Z",Period:"game",QuoteAt:"2026-11-25T18:09:00Z",SourceUpdatedAt:"2026-11-25T18:09:00Z",Odds:-110,Book:"test",AuditVersion:2}
 rows:=makeProgress([]picklog.Entry{entry},"V2",Game{ID:11,Status:"Halftime"},map[int]StatLine{},nil,now.Format(time.RFC3339))
 if len(rows)!=1||rows[0].HasLiveStats{t.Fatalf("missing feed cannot present 0 assists: %#v",rows)}
}
func TestMarketImpliedIsNotV2Probability(t *testing.T) {
 over,under:=noVig(-125,105)
 if !(over>0.5 && over<0.6 && under<0.5){t.Fatalf("incorrect no-vig odds %v %v",over,under)}
}


func TestDedicatedLiveBoxGetsBothTeamStatsWithoutGuessingAbsentPlayers(t *testing.T){
 live:=json.RawMessage(`{
 "date":"2026-11-25","status":"Halftime",
 "home_team":{"id":7,"players":[
  {"player":{"id":101},"pts":12,"reb":3,"ast":4,"fg3m":2},
  {"player":{"id":102},"pts":null,"reb":0,"ast":0,"fg3m":0}]},
 "visitor_team":{"id":16,"players":[
  {"player":{"id":201},"pts":15,"reb":5,"ast":2,"fg3m":1}]}
 }`)
 game,stats,ok:=parseLiveBox(live,map[string]int{"7/16":222})
 if !ok||game.ID!=222||game.Status!="Halftime"||len(stats)!=2{
  t.Fatalf("live box should map home and away and exclude incomplete rows: %#v %#v",game,stats)
 }
 if stats[101].Assists!=4||stats[201].Threes!=1{t.Fatal("incorrect halftime data",stats)}
 if _,_,ok:=parseLiveBox(live,map[string]int{"7/28":222});ok{t.Fatal("unmatched game cannot use another team's live box")}
}
