const fs=require('fs'),path=require('path'),assert=require('node:assert/strict');
const siteDir=path.resolve(__dirname,'..');
const tempDir=fs.mkdtempSync(path.join(require('os').tmpdir(),'wraith-ui-'));
process.env.NODE_PATH=path.join(siteDir,'node_modules');require('module').Module._initPaths();
const esbuild=require('esbuild'),React=require('react');const {act}=React;
const {JSDOM}=require('jsdom');
const dom=new JSDOM('<div id="root"></div>',{url:'https://preview.test/'});
global.window=dom.window;global.document=dom.window.document;global.navigator=dom.window.navigator;global.IS_REACT_ACT_ENVIRONMENT=true;
const {createRoot}=require('react-dom/client');
for(const name of ['Board','Combos'])esbuild.buildSync({entryPoints:[path.join(siteDir,'src',name+'.jsx')],outfile:path.join(tempDir,`${name}.cjs`),bundle:true,platform:'node',format:'cjs',jsx:'automatic',packages:'external'});
const Board=require(path.join(tempDir,'Board.cjs')).default,Combos=require(path.join(tempDir,'Combos.cjs')).default;
const fixture=JSON.parse(fs.readFileSync(path.join(__dirname,'fixtures/board-ui.json')));
// Keep all fixtures fresh regardless of time elapsed since creation.
const now=new Date().toISOString(),tip=new Date(Date.now()+4*3600000).toISOString();
for(const [i,g] of fixture.board.games.entries()){g.tip=new Date(Date.parse(tip)+i*3600000).toISOString();for(const p of [...g.plays,...g.more])Object.assign(p,{tip:g.tip,quoteAt:now,sourceUpdatedAt:now});}
for(const c of fixture.board.combos)Object.assign(c,{tip,quoteAt:now,sourceUpdatedAt:now});
for(const o of fixture.board.overs)o.tip=tip;
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
check('Overs are outside a collapsed disclosure',$$('.ov-head .player-portrait img').length===2&&!$('.ov-head').closest('details'));
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
console.log(JSON.stringify({passed:checks.length,checks},null,2));fs.rmSync(tempDir,{recursive:true,force:true});dom.window.close();
