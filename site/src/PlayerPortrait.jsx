import {useState} from 'react'
import {photoFor,initials} from './photos.js'
export default function PlayerPortrait({name='',team=''}) {
 const [failed,setFailed]=useState(false)
 const url=photoFor(name)
 return <span className={`player-portrait team-${team}`} aria-hidden="true">{url&&!failed?<img src={url} alt="" loading="lazy" decoding="async" referrerPolicy="no-referrer" onError={()=>setFailed(true)}/>:<span className="portrait-initials">{initials(name)}</span>}</span>
}
