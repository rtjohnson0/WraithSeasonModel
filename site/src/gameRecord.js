export function gameRecordRows(rows=[],selected=false) {
 return rows.filter(r=>!selected||r.tier==='PICK')
}
export function summarizeGameRows(rows=[]) {
 return ['moneyline','spread','total'].map(market=>{
 const group=rows.filter(r=>r.market===market),graded=group.filter(r=>['WIN','LOSS'].includes(r.result))
 const wins=graded.filter(r=>r.result==='WIN').length
 return {market,wins,losses:graded.length-wins,pushes:group.filter(r=>r.result==='PUSH').length,pending:group.filter(r=>['PENDING','REVIEW'].includes(r.result)).length,hitRate:graded.length?wins/graded.length:null,units:group.filter(r=>['WIN','LOSS','PUSH'].includes(r.result)).reduce((s,r)=>s+(r.units||0),0)}
 })
}
