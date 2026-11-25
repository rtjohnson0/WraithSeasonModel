package props

import (
    "fmt"
    "os"
    "path/filepath"
    "testing"
    "time"
)

func TestLatestCompletedInjuryCaptureDropsEarlierOutForBothTeams(t *testing.T) {
    dir := t.TempDir()
    date := "2026-11-25"
    earlier := "2026-11-25T17:00:00Z"
    latest := "2026-11-25T17:10:00Z"
    now, _ := time.Parse(time.RFC3339, "2026-11-25T17:15:00Z")
    health := fmt.Sprintf(`{"date":%q,"captured_at":%q,"games":2,"injuries_complete":true}`, date, latest)
    if err := os.WriteFile(filepath.Join(dir,"injury_health_"+date+".json"), []byte(health),0600); err!=nil {t.Fatal(err)}
    // Prior batch had a player Out on the HOME side. The new complete
    // report no longer lists that player; it must not stay marked Out.
    entries := fmt.Sprintf(`{"captured_at":%q,"row":{"status":"Out","player":{"id":101}}}`+"\n",earlier) +
        fmt.Sprintf(`{"captured_at":%q,"row":{"status":"Out","player":{"id":201}}}`+"\n",latest) +
        fmt.Sprintf(`{"captured_at":%q,"row":{"status":"Questionable","player":{"id":102}}}`+"\n",latest)
    if err:=os.WriteFile(filepath.Join(dir,"injuries_"+date+".jsonl"),[]byte(entries),0600);err!=nil {t.Fatal(err)}
    statuses, capture, err := LoadCapturedInjuries(dir,date,now)
    if err!=nil {t.Fatal(err)}
    if capture!=latest || len(statuses)!=2 || statuses[101]!="" || statuses[201]!="Out" || statuses[102]!="Questionable" {
        t.Fatalf("only latest complete batch from home and away must drive injuries: %v, capture=%s",statuses,capture)
    }
}

func TestCurrentInjuryReceiptFailsClosedWhenIncompleteOrStale(t *testing.T) {
    dir := t.TempDir()
    now, _ := time.Parse(time.RFC3339, "2026-11-25T18:00:00Z")
    date:="2026-11-25"
    if _,_,err := LoadCapturedInjuries(dir,date,now);err==nil {t.Fatal("missing receipt must fail")}
    path:=filepath.Join(dir,"injury_health_"+date+".json")
    for _, data := range []string{
        `{"date":"2026-11-25","captured_at":"2026-11-25T17:58:00Z","games":2,"injuries_complete":false}`,
        `{"date":"2026-11-25","captured_at":"2026-11-25T17:00:00Z","games":2,"injuries_complete":true}`,
        `{"date":"2026-11-24","captured_at":"2026-11-25T17:58:00Z","games":2,"injuries_complete":true}`,
    } {
        if err:=os.WriteFile(path,[]byte(data),0600);err!=nil {t.Fatal(err)}
        if _,_,err:=LoadCapturedInjuries(dir,date,now);err==nil {t.Fatalf("unsafe receipt accepted: %s",data)}
    }
}

func TestSuccessfulEmptyInjuryListIsNotInvented(t *testing.T) {
    dir:=t.TempDir()
    date:="2026-11-25"
    now,_:=time.Parse(time.RFC3339,"2026-11-25T18:00:00Z")
    if err:=os.WriteFile(filepath.Join(dir,"injury_health_"+date+".json"), []byte(
        `{"date":"2026-11-25","captured_at":"2026-11-25T17:59:00Z","games":1,"injuries_complete":true}`),0600);err!=nil {t.Fatal(err)}
    status,_,err:=LoadCapturedInjuries(dir,date,now)
    if err!=nil || len(status)!=0 {t.Fatalf("complete empty injury response should be allowed, got %v %v",status,err)}
}
