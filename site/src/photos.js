import roster from './playerPhotos.json' with {type:'json'}
export function playerKey(name='') {return name.normalize('NFD').replace(/\p{M}/gu,'').toLowerCase().replace(/[^a-z0-9]/g,'')}
export function photoFor(name='') {
 const p=roster.players[playerKey(name)]
 return p&&Number.isInteger(p.id)&&p.id>0?`https://cdn.nba.com/headshots/nba/latest/260x190/${p.id}.png`:null
}
export function initials(name=''){return name.trim().split(/\s+/).filter(Boolean).map(s=>s[0]).filter(Boolean).filter((_,i,a)=>i===0||i===a.length-1).join('').toUpperCase().slice(0,2)}
