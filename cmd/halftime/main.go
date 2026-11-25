// Command halftime captures a lightweight in-progress scoreboard and optional
// verified *post-tip* full-game sportsbook quotes for original pregame picks.
// It does not modify pregame pick logs, trained stats or selection rules.
package main

import (
 "encoding/json"
 "errors"
 "flag"
 "fmt"
 "math"
 "net/url"
 "os"
 "path/filepath"
 "sort"
 "strconv"
 "strings"
 "time"
 "propsmodel/internal/bdl"
 "propsmodel/internal/boardstate"
 "propsmodel/internal/data"
 "propsmodel/internal/picklog"
 "propsmodel/internal/props"
)

type Game struct { ID int `json:"id"`; Status string `json:"status"`; Date string `json:"date"` }
type StatLine struct {
 GameID int `json:"game_id"`
 PlayerID int `json:"player_id"`
 Period int `json:"period"`
 Minutes string `json:"minutes,omitempty"`
 Points float64 `json:"points"`
 Rebounds float64 `json:"rebounds"`
 Assists float64 `json:"assists"`
 Threes float64 `json:"threes"`
}
type Offer struct {
 Book string `json:"book"`
 Line float64 `json:"line"`
 OverOdds int `json:"overOdds"`
 UnderOdds int `json:"underOdds"`
 OverMarketProb float64 `json:"overMarketProb"`
 UnderMarketProb float64 `json:"underMarketProb"`
 UpdatedAt string `json:"updatedAt"`
}
type Progress struct {
 Model string `json:"model"`
 GameID int `json:"gameId"`
 Player string `json:"player"`
 Stat string `json:"stat"`
 OriginalLine float64 `json:"originalLine"`
 OriginalSide string `json:"originalSide"`
 OriginalOdds int `json:"originalOdds"`
 OriginalProb float64 `json:"originalPregameProb"`
 OriginalAt string `json:"originalAt"`
 Tip string `json:"tip"`
 Matchup string `json:"matchup"`
 GameStatus string `json:"gameStatus"`
 Current float64 `json:"current"`
 RemainingToOver int `json:"remainingToOver"`
 AtOrAboveLine bool `json:"atOrAboveLine"`
 HasLiveStats bool `json:"hasLiveStats"`
 Live *Offer `json:"live,omitempty"`
}
type Report struct { Schema int `json:"schema"`; Date string `json:"date"`; CapturedAt string `json:"capturedAt"`; DataStatus string `json:"dataStatus"`; Plays []Progress `json:"plays"` }

func probability(american int) float64 {
 if american > 0 {return 100/float64(100+american)}
 if american < 0 {return float64(-american)/float64(100-american)}
 return 0
}
func noVig(over,under int)(float64,float64) {
 a,b:=probability(over),probability(under)
 if a==0||b==0{return 0,0}
 return a/(a+b),b/(a+b)
}
func thresholdRemaining(line,current float64) int {
 // Strict O: at 6.5 with 4 assists -> 3 more; at 7 with 4 -> 4 more.
 r:=int(math.Floor(line-current))+1
 if r<0{return 0}
 return r
}
func running(status string) bool {
 s:=strings.ToLower(strings.TrimSpace(status))
 if s==""||strings.Contains(s,"final")||strings.Contains(s,"postpon")||strings.Contains(s,"cancel")||strings.Contains(s,"scheduled")||strings.Contains(s,"pre-game")||s=="not started" {return false}
 return strings.Contains(s,"half")||strings.Contains(s,"quarter")||strings.Contains(s,"qtr")||strings.Contains(s,"in progress")||strings.Contains(s,"end of")||strings.Contains(s,"overtime")||strings.Contains(s," ot")
}
func halftime(status string) bool {return strings.Contains(strings.ToLower(status),"half")}
func statValue(s StatLine, stat string)float64 {
 switch stat {case "points":return s.Points;case "rebounds":return s.Rebounds;case "assists":return s.Assists;case "threes":return s.Threes}
 return 0
}
func parseStats(raw json.RawMessage)(StatLine,bool){
 var r struct{
  Game struct {ID int `json:"id"`} `json:"game"`
  Player struct{ID int `json:"id"`} `json:"player"`
  Period *int `json:"period"`
  Min json.RawMessage `json:"min"`
  Pts *float64 `json:"pts"`
  Reb *float64 `json:"reb"`
  Ast *float64 `json:"ast"`
  Fg3m *float64 `json:"fg3m"`
 }
 if json.Unmarshal(raw,&r)!=nil||r.Game.ID<=0||r.Player.ID<=0||r.Pts==nil||r.Reb==nil||r.Ast==nil||r.Fg3m==nil {return StatLine{},false}
 if r.Period!=nil&&*r.Period!=0{return StatLine{},false} // never mix 1Q with cumulative box score
 return StatLine{GameID:r.Game.ID,PlayerID:r.Player.ID,Points:*r.Pts,Rebounds:*r.Reb,Assists:*r.Ast,Threes:*r.Fg3m},true
}
func parseLiveOffers(raw []byte,gameID int,now,tip time.Time)map[string]props.Line{
 best:=map[string]props.Line{}
 // BDL can return pregame quotes after tip. Those must NOT masquerade as live.
 for _,q:=range props.ParseResponse(raw,gameID){
  updated,err:=time.Parse(time.RFC3339,q.UpdatedAt)
  if err!=nil||!updated.After(tip)||updated.After(now.Add(60*time.Second))||now.Sub(updated)>3*time.Minute||!q.QuoteVerified||!props.ValidOdds(q.OverOdds)||!props.ValidOdds(q.UnderOdds){continue}
  if q.GameID!=gameID||!q.Stat.IsCombo()&&q.Stat!=data.Points&&q.Stat!=data.Rebounds&&q.Stat!=data.Assists&&q.Stat!=data.Threes {continue}
  key:=fmt.Sprintf("%d:%s",q.PlayerID,q.Stat)
  before,exists:=best[key]
  if !exists || math.Abs(probability(q.OverOdds)-probability(q.UnderOdds))<math.Abs(probability(before.OverOdds)-probability(before.UnderOdds)) {best[key]=q}
 }
 return best
}
func makeProgress(entries []picklog.Entry,model string,game Game,stats map[int]StatLine,offers map[string]props.Line,at string)[]Progress{
 out:=[]Progress{}
 for _,p:=range entries{
  if p.GameID!=game.ID||!p.FullGame()||!p.Verified()||p.PlayerID<=0 {continue}
  tip,te:=time.Parse(time.RFC3339,p.Tip)
  captured,ce:=time.Parse(time.RFC3339,p.LoggedAt)
  if te!=nil||ce!=nil||!captured.Before(tip) {continue}
  s,exists:=stats[p.PlayerID]
  progress:=Progress{Model:model,GameID:game.ID,Player:p.Player,Stat:p.Stat,OriginalLine:p.Line,OriginalSide:p.Side,OriginalOdds:p.Odds,OriginalProb:p.Prob,OriginalAt:p.LoggedAt,Tip:p.Tip,Matchup:p.Matchup,GameStatus:game.Status,HasLiveStats:exists}
  if exists {
   progress.Current=statValue(s,p.Stat)
   progress.RemainingToOver=thresholdRemaining(p.Line,progress.Current)
   progress.AtOrAboveLine=progress.Current>=p.Line
  }
  if q,found:=offers[fmt.Sprintf("%d:%s",p.PlayerID,p.Stat)];found&&exists {
   over,under:=noVig(q.OverOdds,q.UnderOdds)
   progress.Live=&Offer{Book:q.Vendor,Line:q.Line,OverOdds:q.OverOdds,UnderOdds:q.UnderOdds,OverMarketProb:over,UnderMarketProb:under,UpdatedAt:q.UpdatedAt}
  }
  out=append(out,progress)
 }
 return out
}
func run(c *bdl.Client,dir,date string,now time.Time)(Report,error){
 snapshot,err:=boardstate.Load(dir)
 if err!=nil{return Report{},err}
 if snapshot==nil||snapshot.Date!=date{return Report{},errors.New("no matching saved pregame board; do not reconstruct from a new live book")}
 var v2 struct {Date string `json:"date"`;Plays []picklog.Entry `json:"plays"`}
 if raw,err:=os.ReadFile(filepath.Join(dir,"v2.json"));err==nil{_ = json.Unmarshal(raw,&v2)}
 report:=Report{Schema:1,Date:date,CapturedAt:now.UTC().Format(time.RFC3339),DataStatus:"Live game boxes; market probabilities are sportsbook-implied, not model predictions",Plays:[]Progress{}}
 var league struct{Data []struct{ID int `json:"id"`;Status string `json:"status"`;Date string `json:"date"`} `json:"data"`}
 q:=url.Values{"dates[]":{date}}
 raw,err:=c.Get("/nba/v1/games",q)
 if err!=nil{return report,err}
 if err=json.Unmarshal(raw,&league);err!=nil{return report,err}
 saved:=map[int]bool{}
 for _,g:=range snapshot.Games{saved[g.ID]=true}
 for _,g:=range league.Data{
  if !saved[g.ID]||!running(g.Status){continue}
  game:=Game{ID:g.ID,Status:g.Status,Date:g.Date}
  stats:=map[int]StatLine{}
  query:=url.Values{"game_ids[]":{strconv.Itoa(g.ID)}}
  err:=c.Paginate("/nba/v1/stats",query,func(raw json.RawMessage)error{
   stat,ok:=parseStats(raw);if ok&&stat.GameID==g.ID{stats[stat.PlayerID]=stat};return nil
  })
  if err!=nil{continue} // never show an unverified 0 as a live score
  offers:=map[string]props.Line{}
  if halftime(g.Status){
   // One small quote request; unavailable/closed/stale live markets are omitted.
   if raw,e:=c.Get("/nba/v2/odds/player_props",url.Values{"game_id":{strconv.Itoa(g.ID)}});e==nil{
    tip:=now
    for _,item:=range snapshot.Games{if item.ID==g.ID{if v,e:=time.Parse(time.RFC3339,item.Tip);e==nil{tip=v}}}
    offers=parseLiveOffers(raw,g.ID,now,tip)
   }
  }
  report.Plays=append(report.Plays,makeProgress(snapshot.Plays,"Board",game,stats,offers,report.CapturedAt)...)
  if v2.Date==date{report.Plays=append(report.Plays,makeProgress(v2.Plays,"V2",game,stats,offers,report.CapturedAt)...)}
 }
 sort.SliceStable(report.Plays,func(i,j int)bool{
  a,b:=report.Plays[i],report.Plays[j]
  if halftime(a.GameStatus)!=halftime(b.GameStatus){return halftime(a.GameStatus)}
  if (a.Live!=nil)!=(b.Live!=nil){return a.Live!=nil}
  if a.Live!=nil&&b.Live!=nil{
   ap:=math.Max(a.Live.OverMarketProb,a.Live.UnderMarketProb)
   bp:=math.Max(b.Live.OverMarketProb,b.Live.UnderMarketProb)
   if ap!=bp{return ap>bp}
  }
  if a.GameID!=b.GameID{return a.GameID<b.GameID}
  return a.Player<b.Player
 })
 return report,nil
}
func main(){
 dataDir:=flag.String("data","data","directory holding pregame boards")
 out:=flag.String("out","data/halftime.json","site data report output")
 flag.Parse()
 key:=os.Getenv("BALLDONTLIE_API_KEY")
 if key==""{fmt.Fprintln(os.Stderr,"missing BALLDONTLIE_API_KEY");os.Exit(1)}
 loc,_:=time.LoadLocation("America/New_York")
 now:=time.Now()
 report,err:=run(bdl.New(key),*dataDir,now.In(loc).Format("2006-01-02"),now)
 if err!=nil{fmt.Fprintln(os.Stderr,err);os.Exit(1)}
 if err=os.MkdirAll(filepath.Dir(*out),0755);err!=nil{panic(err)}
 b,err:=json.MarshalIndent(report,"","  ");if err!=nil{panic(err)}
 if err=os.WriteFile(*out,append(b,'\n'),0644);err!=nil{panic(err)}
 fmt.Printf("Live halftime board: %d pregame props with verified current stats, saved %s\n",len(report.Plays),*out)
}
