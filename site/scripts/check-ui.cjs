const fs=require('fs'),path=require('path'),assert=require('node:assert/strict');
const siteDir=path.resolve(__dirname,'..');
const tempDir=fs.mkdtempSync(path.join(require('os').tmpdir(),'wraith-ui-'));
process.env.NODE_PATH=path.join(siteDir,'node_modules');require('module').Module._initPaths();
const esbuild=require('esbuild'),React=require('react');const {act}=React;
const {JSDOM}=require('jsdom');
const dom=new JSDOM('<div id="root"></div>',{url:'https://preview.test/'});
global.window=dom.window;global.document=dom.window.document;global.navigator=dom.window.navigator;global.IS_REACT_ACT_ENVIRONMENT=true;
const {createRoot}=require('react-dom/client');
for(const name of ['Board','Combos','Record'])esbuild.buildSync({entryPoints:[path.join(siteDir,'src',name+'.jsx')],outfile:path.join(tempDir,`${name}.cjs`),bundle:true,platform:'node',format:'cjs',jsx:'automatic',packages:'external'});
const Board=require(path.join(tempDir,'Board.cjs')).default,Combos=require(path.join(tempDir,'Combos.cjs')).default;
const Record=require(path.join(tempDir,'Record.cjs')).default;
const fixture=JSON.parse(fs.readFileSync(path.join(__dirname,'fixtures/board-ui.json')));
// Keep all fixtures fresh regardless of time elapsed since creation.
const now=new Date().toISOString(),tip=new Date(Date.now()+4*3600000).toISOString();
for(const [i,g] of fixture.board.games.entries()){g.tip=new Date(Date.parse(tip)+i*3600000).toISOString();for(const p of [...g.plays,...g.more])Object.assign(p,{tip:g.tip,quoteAt:now,sourceUpdatedAt:now});}
for(const c of fixture.board.combos)Object.assign(c,{tip,quoteAt:now,sourceUpdatedAt:now});
for(const o of fixture.board.overs) {
 o.tip=tip
 o.analysis={...structuredClone(fixture.board.games[0].plays[0]),player:o.player,team:o.team,stat:o.stat,side:'OVER',line:o.line,projStat:o.proj,projMin:o.proj_min,prob:o.prob,edge:o.edge,odds:o.odds,book:o.book,quoteAt:now,sourceUpdatedAt:now,tip,research:true,label:'Research comparison',grade:'—'}
}
let root=createRoot(document.getElementById('root'));const checks=[];
function check(label,condition){assert.ok(condition,label);checks.push(label);}
function click(el){assert.ok(el,'Control exists');act(()=>el.dispatchEvent(new window.MouseEvent('click',{bubbles:true})));}
function input(el,value){act(()=>{const setter=Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype,'value').set;setter.call(el,value);el.dispatchEvent(new window.Event('input',{bubbles:true}));});}
const $=s=>document.querySelector(s),$$=s=>[...document.querySelectorAll(s)];
fixture.board.games[0].plays[0].last5[0].value=fixture.board.games[0].plays[0].line;
act(()=>root.render(React.createElement(Board,{site:fixture})));
check('V1 groups keep games in scheduled order',$$('.timed-game .schedule-divider h3').map(n=>n.textContent).join('|')==='GSW @ LAL|DAL @ BOS');
check('Every V1 card shows a prominent line ticket',$$('.prop-grid .prop-ticket').length===5&&$('.prop-grid .ticket-line').textContent==='25.5');
check('Recent form exposes numeric outcomes',$$('.prop-grid>.play')[0].querySelectorAll('.form-result').length===5);
check('A historical result exactly on the line is shown as a push',$$('.prop-grid>.play')[0].querySelector('.form-result.push b').textContent==='25.5');
check('Every V1 prop card has a player image URL',$$('.prop-grid>.play').length===5&&$$('.prop-grid>.play .player-portrait img').length===5);
act(()=>$('.prop-grid .player-portrait img').dispatchEvent(new window.Event('error')));
check('Unavailable photos fall back to player initials',$('.prop-grid .portrait-initials')?.textContent==='SC');
check('Games no longer compete with props in a separate section',!$('.boardpage>.game-markets:not(.quarter-board)'));
check('Over comparisons have player photos and expanded analytics',$$('#overs-watch .player-portrait img').length===2&&$$('#overs-watch .why .bars').length===4);
check('Over comparisons never look like selected recommendations',$$('#overs-watch .tier').every(el=>el.textContent==='Research comparison'));
check('Over comparisons include quote details and matchup explanations',$$('#overs-watch .pick-reason').length===2&&$('#overs-watch').textContent.includes('How defense affects this forecast'));
click($('.prop-grid .playbtn'));check('Expanded V1 analysis retains charts',$$('.prop-grid .why .bars').length===2);
click($('.prop-grid .matchup-lens>summary'));check('Game context attached to correct card',$('.prop-grid .matchup-lens').textContent.includes('GSW ML'));
click($('[aria-label="Side"] button[aria-pressed="false"]'));check('Over filter shows only overs',$$('.prop-grid>.play').every(el=>el.classList.contains('over')));
click($$('.filter-summary button').find(b=>b.textContent==='Clear filters'));
input($('input[type="search"]'),'Curry');check('V1 player search filters cards',$$('.prop-grid>.play').length===1&&$('.prop-grid>.play').textContent.includes('Stephen Curry'));
input($('input[type="search"]'),'UnknownPlayer');check('No-result state renders',$$('.prop-grid>.play').length===0&&document.body.textContent.includes('No plays match'));
click($$('.filter-summary button').find(b=>b.textContent==='Clear filters'));check('Clear filters restores all cards',$$('.prop-grid>.play').length===5);
const closed=structuredClone(fixture);closed.board.games[0].tip=new Date(Date.now()-3600000).toISOString();for(const p of closed.board.games[0].plays)p.tip=closed.board.games[0].tip;
act(()=>root.render(React.createElement(Board,{site:closed})));check('Started games retain cards with closed labels',$$('.prop-grid>.play').length===5&&$$('.card-quote-state').some(el=>el.textContent.includes('Game started')));
act(()=>root.unmount());root=createRoot($('#root'));
act(()=>root.render(React.createElement(Combos,{site:fixture})));
check('Every selected combo card has a player image URL',$$('.combo-grid .player-portrait img').length===2);
click($('.combo-summary'));check('Expanded combo retains charts',$('.combo-grid>.combo-row').querySelectorAll('.combo-expanded .bars').length===2);
click($('.combo-grid .matchup-lens>summary'));check('Legacy combos resolve game by matchup',$('.combo-grid .matchup-lens').textContent.includes('GSW ML'));
click($$('[aria-label="View"] button').find(b=>b.textContent.includes('Line comparisons')));
check('Research comparison shows projected total instead of raw probability',$('.combo-prob').textContent.includes('37.3')&&!$('.combo-prob').textContent.includes('81'));
check('Research comparison has player image',$$('.combo-grid .player-portrait img').length===1);
input($('input[type="search"]'),'UnknownPlayer');check('Combo empty search state',$$('.combo-grid>.combo-row').length===0&&document.body.textContent.includes('No combo plays match'));
act(()=>root.unmount());
root=createRoot($('#root'));
const recordFixture={record:{since:'2026-11-14',until:'2026-11-15',days:2,tiers:[{name:'Everything tracked',wins:2,losses:1,pushes:0,hitRate:2/3,predicted:.6,units:.8}],stats:[],sides:[],daily:[],buckets:[],recent:[
 {date:'2026-11-15',player:'Stephen Curry',team:'GSW',matchup:'GSW @ LAL',stat:'points',side:'OVER',line:20.5,actual:25,proj:24,result:'WIN',tier:'PAPER'},
 {date:'2026-11-15',player:'Kevin Durant',team:'HOU',matchup:'HOU @ WAS',stat:'assists',side:'UNDER',line:4.5,actual:6,proj:4,result:'LOSS',tier:'LEAN'},
 {date:'2026-11-14',player:'Devin Booker',team:'PHX',stat:'threes',side:'OVER',line:1.5,actual:3,proj:2,result:'WIN',tier:'PAPER'}]},gameRecord:[
 {date:'2026-11-17',game_id:17,home:'LAL',away:'GSW',market:'moneyline',side:'LAL',line:0,odds:-110,prob:.55,book:'test',tier:'PAPER',result:'PENDING',units:0},
 {date:'2026-11-15',game_id:15,home:'LAL',away:'GSW',market:'spread',side:'LAL',line:-2.5,odds:-110,prob:.55,book:'test',tier:'PAPER',result:'WIN',units:.91,homeScore:100,awayScore:90},
 {date:'2026-11-14',game_id:14,home:'LAL',away:'GSW',market:'moneyline',side:'LAL',line:0,odds:-110,prob:.55,book:'test',tier:'PICK',result:'WIN',units:.91,homeScore:100,awayScore:90}]};
act(()=>root.render(React.createElement(Record,{site:recordFixture})));
check('Individual prop results precede game markets',Boolean($('.prop-results').compareDocumentPosition($('.game-record'))&window.Node.DOCUMENT_POSITION_FOLLOWING));
check('Prop cards retain player portraits, final actuals and projections',$$('.record-page>.prop-record .prop-result-cards>li').length>0&&$('.prop-result-cards>li .player-portrait')&&$('.prop-result-cards>li .result-values').textContent.includes('Final result'));
const latestPropDate=[...new Set(recordFixture.record.recent.map(r=>r.date))].sort().reverse()[0];
check('Latest completed prop date is selected by default',$('.prop-results select').value===latestPropDate);
click($$('.prop-results button').find(b=>b.textContent==='Hits'));
check('Prop Hits filter excludes losses',$$('.record-page>.prop-record .prop-result-cards>li').length>0&&$$('.record-page>.prop-record .prop-result-cards>li').every(n=>n.classList.contains('win')));
click($$('.prop-results button').find(b=>b.textContent==='All'));
input($('.prop-results input'),'UnknownPlayer');check('Prop result search has a clear empty state',$$('.record-page>.prop-record .prop-result-cards>li').length===0&&$('.prop-results').textContent.includes('No prop results match'));
input($('.prop-results input'),'');
act(()=>{$('.prop-results select').value='ALL';$('.prop-results select').dispatchEvent(new window.Event('change',{bubbles:true}));});
check('All dates exposes every available prop actual',$$('.record-page>.prop-record .prop-result-cards>li').length===recordFixture.record.recent.length);
check('Charts remain accessible in a breakdown disclosure',$('.record-analysis').textContent.includes('Units over time'));
check('Each game market has a summary card',$$('.game-record-summary article').length===3);
const latestGradedGameDate=[...new Set(recordFixture.gameRecord.filter(r=>['WIN','LOSS','PUSH'].includes(r.result)).map(r=>r.date))].sort().reverse()[0];
check('Game results default to the latest completed date',$('.game-record select').value===latestGradedGameDate);
click($$('.game-record button').find(b=>b.textContent==='Selected plays only'));
check('Selected game view never labels paper as selected',$$('.game-result').every(n=>!n.textContent.includes('not staked')));
act(()=>root.unmount());
act(()=>{root=createRoot($('#root'));root.render(React.createElement(Record,{site:{...recordFixture,hideGameRecord:true}}));});
check('Embedded research pages can still hide game records',!$('.game-record'));
act(()=>root.unmount());
console.log(JSON.stringify({passed:checks.length,checks},null,2));fs.rmSync(tempDir,{recursive:true,force:true});dom.window.close();
