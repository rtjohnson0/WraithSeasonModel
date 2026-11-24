const fs=require('fs'),path=require('path'),assert=require('node:assert/strict');
const siteDir=path.resolve(__dirname,'..');
const tempDir=fs.mkdtempSync(path.join(require('os').tmpdir(),'wraith-ui-'));
process.env.NODE_PATH=path.join(siteDir,'node_modules');require('module').Module._initPaths();
const esbuild=require('esbuild'),React=require('react');const {act}=React;
const {JSDOM}=require('jsdom');
const dom=new JSDOM('<div id="root"></div>',{url:'https://preview.test/'});
global.window=dom.window;global.document=dom.window.document;global.navigator=dom.window.navigator;global.IS_REACT_ACT_ENVIRONMENT=true;
const {createRoot}=require('react-dom/client');
for(const name of ['Board','Combos','Record','Rotation','GamePace'])esbuild.buildSync({entryPoints:[path.join(siteDir,'src',name+'.jsx')],outfile:path.join(tempDir,`${name}.cjs`),bundle:true,platform:'node',format:'cjs',jsx:'automatic',packages:'external'});
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
fixture.board.games[0].plays[0].last10=[...structuredClone(fixture.board.games[0].plays[0].last5),...structuredClone(fixture.board.games[0].plays[0].last5)];
act(()=>root.render(React.createElement(Board,{site:fixture})));
check('V1 groups keep games in scheduled order',$$('.timed-game .schedule-divider h3').map(n=>n.textContent).join('|')==='GSW @ LAL|DAL @ BOS');
check('Every V1 card shows a prominent line ticket',$$('.timed-game .prop-grid .prop-ticket').length===5&&$('.timed-game .prop-grid .ticket-line').textContent==='25.5');
check('Recent form exposes numeric outcomes',$$('.timed-game .prop-grid>.play')[0].querySelectorAll('.form-result').length===5);
check('A historical result exactly on the line is shown as a push',$$('.timed-game .prop-grid>.play')[0].querySelector('.form-result.push b').textContent==='25.5');
click($$('.timed-game .prop-grid>.play')[0].querySelectorAll('.form-range button')[1]);
check('Last 10 displays ten recent results',$$('.timed-game .prop-grid>.play')[0].querySelectorAll('.form-result').length===10);
click($$('.timed-game .prop-grid>.play')[0].querySelectorAll('.form-range button')[0]);
check('Last 5 restores five recent results',$$('.timed-game .prop-grid>.play')[0].querySelectorAll('.form-result').length===5);
check('Every V1 prop card has a player image URL',$$('.timed-game .prop-grid>.play').length===5&&$$('.timed-game .prop-grid>.play .player-portrait img').length===5);
act(()=>$('.timed-game .prop-grid .player-portrait img').dispatchEvent(new window.Event('error')));
check('Unavailable photos fall back to player initials',$('.timed-game .prop-grid .portrait-initials')?.textContent==='SC');
check('Games no longer compete with props in a separate section',!$('.boardpage>.game-markets:not(.quarter-board)'));
check('Over comparisons use the shared prop-card grid',$('#overs-watch > ul').classList.contains('prop-grid'));
check('Over research starts collapsed',!$('#overs-watch').open&&$$('#overs-watch .playbtn').every(b=>b.getAttribute('aria-expanded')==='false'));
for(const b of $$('#overs-watch .playbtn'))click(b);
check('Over comparisons have player photos and optional analytics',$$('#overs-watch .player-portrait img').length===2&&$$('#overs-watch .why .bars').length===4);
check('Over comparisons never look like selected recommendations',$$('#overs-watch .tier').every(el=>el.textContent==='Research comparison'));
check('Over comparisons include quote details and matchup explanations',$$('#overs-watch .pick-reason').length===2&&$('#overs-watch').textContent.includes('How defense affects this forecast'));
click($('.timed-game .prop-grid .playbtn'));check('Expanded V1 analysis retains charts',$$('.timed-game .prop-grid .why .bars').length===2);
click($('.timed-game .prop-grid .matchup-lens>summary'));check('Game context attached to correct card',$('.timed-game .prop-grid .matchup-lens').textContent.includes('GSW ML'));
click($('[aria-label="Side"] button[aria-pressed="false"]'));check('Over filter shows only overs',$$('.timed-game .prop-grid>.play').every(el=>el.classList.contains('over')));
click($$('.filter-summary button').find(b=>b.textContent==='Clear filters'));
input($('input[type="search"]'),'Curry');check('V1 player search filters cards',$$('.timed-game .prop-grid>.play').length===1&&$('.timed-game .prop-grid>.play').textContent.includes('Stephen Curry'));
input($('input[type="search"]'),'UnknownPlayer');check('No-result state renders',$$('.timed-game .prop-grid>.play').length===0&&document.body.textContent.includes('No plays match'));
click($$('.filter-summary button').find(b=>b.textContent==='Clear filters'));check('Clear filters restores all cards',$$('.timed-game .prop-grid>.play').length===5);
const closed=structuredClone(fixture);closed.board.games[0].tip=new Date(Date.now()-3600000).toISOString();for(const p of closed.board.games[0].plays)p.tip=closed.board.games[0].tip;
act(()=>root.render(React.createElement(Board,{site:closed})));check('Started games retain cards with closed labels',$$('.timed-game .prop-grid>.play').length===5&&$$('.card-quote-state').some(el=>el.textContent.includes('Game started')));
act(()=>root.unmount());root=createRoot($('#root'));
const v2Board=structuredClone(fixture);v2Board.modelLabel='V2';v2Board.board.games=[{...v2Board.board.games[0],more:[],plays:v2Board.board.games[0].plays.slice(0,2)}];v2Board.board.overs=[];
v2Board.board.games[0].plays[0].rotation={policy:'rotation-v4'};v2Board.board.games[0].plays[0].projMin=12;v2Board.board.games[0].plays[0].minTypical=28;v2Board.board.games[0].plays[0].tier='V2LEAN';v2Board.board.games[0].plays[0].prob=.56;v2Board.board.games[0].plays[1].tier='CHECK';v2Board.board.games[0].plays[1].prob=.69;
act(()=>root.render(React.createElement(Board,{site:v2Board})));
check('Workload-sensitive V2 leans remain visible with a minutes watch',$('.minutes-watch')?.textContent.includes('12 projected versus 28 recent typical'));
check('V2 board identifies its own model',$('.hero-copy').textContent.includes('NBA INTELLIGENCE / V2'));
check('V2 defaults to backend model order',$('select').value==='model'&&$('.timed-game .prop-grid>.play').textContent.includes(v2Board.board.games[0].plays[0].player));
check('V2 board leans are counted separately from extra leans',$('.hero-stats').textContent.includes('Leans on board01')&&$('.sort-control').parentElement.textContent.includes('1 board leans · 0 extra leans'));
act(()=>{$('select').value='probability';$('select').dispatchEvent(new window.Event('change',{bubbles:true}));});
check('Optional probability sorting remains available',$('.timed-game .prop-grid>.play').textContent.includes(v2Board.board.games[0].plays[1].player));
input($('input[type="search"]'),v2Board.board.games[0].plays[1].player);
check('Filtered V2 lean counts describe the visible cards',$('.sort-control').parentElement.textContent.includes('0 board leans · 0 extra leans'));
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
const Rotation=require(path.join(tempDir,'Rotation.cjs')).default;
const rolePlay={player:'Test Big',stat:'rebounds',side:'OVER',rotation:{policy:'rotation-v3',supported:false,allocationApplied:true,baselineMinutes:28,usedMinutes:32,matchedGames:0,reason:'Projected replacements',ownReplacements:[{playerId:2,player:'Replacement Center',position:'C',baseline:20,projected:30}],production:[{kind:'opponent-allowance',team:1,reason:'League-pooled projected rotation',missing:[{playerId:1,player:'Missing Center',position:'C',games:10,minutes:30,points:12,rebounds:10,assists:2,blocks:2}],effects:[{stat:'rebounds',method:'pooled-lineup',applied:true,scale:1.03,role:{games:12000,days:100}}]}],rateScale:[1,1.03,1,1]}};
act(()=>{root=createRoot($('#root'));root.render(React.createElement(Rotation,{p:rolePlay}));});
check('V2 projected replacements show named roles and baseline/projected minutes',$('#root').textContent.includes('Replacement Center · C')&&$('#root').textContent.includes('20.0')&&$('#root').textContent.includes('30.0'));
check('V2 pooled evidence does not masquerade as zero joint-out games',$('#root').textContent.includes('12000 league player-games / 100 dates')&&$('#root').textContent.includes('+3.0%'));
check('Projected lineups are explicitly distinguished from confirmed starters',$('#root').textContent.includes('not confirmed starters'));
act(()=>root.unmount());
const trackingPlay=structuredClone(rolePlay);trackingPlay.rotation.policy='rotation-v4';trackingPlay.rotation.opportunities=[{stat:'assists',games:3,chances:30,minutes:90,conversion:.5,available:true,through:'2026-11-17'},{stat:'rebounds',games:0,chances:0,available:false,reason:'No recent covered games'}];trackingPlay.rotation.pregameLineups=[{game_id:7,team_id:1,teamAbbr:'BOS',status:'expected',lineup:['PG','SG','SF','PF','C'].map((position,i)=>({position,player:{id:i+1,first_name:'Test',last_name:'Player '+i}}))}];
trackingPlay.rotation.workloadReview={policy:'minutes-sensitivity-v1',directionChanges:true,baselineMinutes:30,baselineProbability:.43};trackingPlay.rotation.workloadScenarios=[{player:'Primary minutes forecast',minutes:30,projection:8,probability:.43}];
act(()=>{root=createRoot($('#root'));root.render(React.createElement(Rotation,{p:trackingPlay}));});
check('V2 tracking distinguishes potential-assist volume from conversion',$('.context-opportunities').textContent.includes('10.0 Potential assists/game')&&$('.context-opportunities').textContent.includes('50.0%'));
check('Missing rebound tracking stays unavailable',$('.context-opportunities').textContent.includes('No recent covered games')&&$('.context-opportunities').textContent.includes('Unavailable'));
check('Expected starters are not labelled provider confirmed',$('.context-opportunities').textContent.includes('Expected starters')&&!$('.context-opportunities').textContent.includes('Provider-confirmed starting five'));
check('Each pregame starter retains a portrait or initials',$$('.context-starters .player-portrait').length===5);
check('A minutes-sensitive lean explains its direction reversal and alternative estimate',$('.context-opportunities').textContent.includes('Minutes-sensitive lean')&&$('.context-opportunities').textContent.includes('43.0%')&&$('.context-opportunities').textContent.includes('Primary minutes forecast'));
act(()=>root.unmount());
const GamePace=require(path.join(tempDir,'GamePace.cjs')).default;
const paceFixture=structuredClone(fixture);paceFixture.gameChallenger={policy:'game-pace-v1',date:paceFixture.board.date,at:new Date().toISOString(),z:{},calibration:{moneyline:{N:12,Dates:2,Ready:false}},games:[{...paceFixture.gameMarkets.games[0],paceContext:{forecast:{Pace:99,HomeEff:1.15,AwayEff:1.1},lineupApplied:true,reason:'Fresh projected rotation',scenarios:[{team:1,playerId:7,player:'Test Guard',margin:4,total:224}]}}]};
act(()=>{root=createRoot($('#root'));root.render(React.createElement(GamePace,{site:paceFixture}));});
check('V2 game markets retain access to the original ML spread and total model',$('.current-game-model')?.textContent.includes('Moneyline'));
check('Game challenger has explicit experimental status',$('#root').textContent.includes('Research estimates · separate record · no stakes'));
check('Game challenger compares possessions and efficiency against baseline',$('#root').textContent.includes('99.0')&&$('#root').textContent.includes('115.0')&&$('#root').textContent.includes('challenger versus current game model'));
check('Lineup scenarios show a player name instead of internal IDs',$('#root').textContent.includes('Test Guard')&&!$('#root').textContent.includes('Player ID out'));
check('Moneyline calibration cannot borrow the prop record',$('#root').textContent.includes('12 / 2')&&$('#root').textContent.includes('Experimental estimate'));
check('Challenger game directions are not labeled selected bets',!$$('.challenger-markets .market-play').some(el=>el.textContent.includes('Selected play')));
act(()=>root.unmount());
console.log(JSON.stringify({passed:checks.length,checks},null,2));fs.rmSync(tempDir,{recursive:true,force:true});dom.window.close();
