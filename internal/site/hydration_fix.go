package site

// hydrationFixUI mantiene las URLs del CDN pasando por el mirror incluso tras
// la hidratación de SvelteKit y reinyecta las acciones locales de la ficha.
const hydrationFixUI = `<script>(function(){
var CDN='https://cdn.animeav1.com/';
var scheduled=false;
function localizeValue(v){if(!v||v.indexOf(CDN)===-1)return v;return v.split(CDN).join('/_cdn/')}
function fixAttrs(root){
  var nodes=[];if(root&&root.nodeType===1)nodes.push(root);
  if(root&&root.querySelectorAll)nodes=nodes.concat(Array.prototype.slice.call(root.querySelectorAll('[src],[srcset],[poster],[data-src],[data-lazy-src]')));
  nodes.forEach(function(el){['src','srcset','poster','data-src','data-lazy-src'].forEach(function(a){var v=el.getAttribute&&el.getAttribute(a),n=localizeValue(v);if(v&&n!==v)el.setAttribute(a,n)})})
}
function shareButton(){
  return Array.prototype.slice.call(document.querySelectorAll('button,a')).find(function(el){var t=(el.textContent||'').trim(),a=(el.getAttribute('aria-label')||'')+' '+(el.getAttribute('title')||'');return /compartir/i.test(t)||/compartir|share/i.test(a)})
}
function iconButton(id,ref,title,svg){var b=document.createElement('button');b.id=id;b.type='button';b.className=ref.className;b.title=title;b.setAttribute('aria-label',title);b.innerHTML=svg;b.style.marginLeft='4px';return b}
function ensureActions(){
  try{
    var m=location.pathname.match(/^\/media\/([^/]+)\/?$/);if(!m)return;var share=shareButton();if(!share)return;
    if(!document.getElementById('mirror-refresh-series')){
      var b=iconButton('mirror-refresh-series',share,'Actualizar esta ficha desde AnimeAV1','<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M20 11a8.1 8.1 0 0 0-15.5-2M4 4v5h5"/><path d="M4 13a8.1 8.1 0 0 0 15.5 2M20 20v-5h-5"/></svg>');
      b.addEventListener('click',async function(){if(b.disabled)return;b.disabled=true;b.classList.add('mirror-refreshing');try{var r=await fetch('/__mirror/refresh?path='+encodeURIComponent(location.pathname),{method:'POST',headers:{'X-Mirror-Action':'refresh-series'}});if(!r.ok)throw new Error(await r.text());location.reload()}catch(e){b.disabled=false;b.classList.remove('mirror-refreshing');alert('No se pudo actualizar la ficha: '+e.message)}});share.insertAdjacentElement('afterend',b)
    }
    if(!document.getElementById('mirror-download-series')){
      var anchor=document.getElementById('mirror-refresh-series')||share;
      var d=iconButton('mirror-download-series',share,'Descargar todos los episodios MP4 al NAS','<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M12 3v12"/><path d="m7 10 5 5 5-5"/><path d="M5 21h14"/></svg>');
      function poll(){fetch('/api/download-status?slug='+encodeURIComponent(m[1]),{cache:'no-store'}).then(function(r){if(!r.ok)throw 0;return r.json()}).then(function(x){d.disabled=!!x.running;d.classList.toggle('mirror-downloading',!!x.running);d.title=x.running?('Descargando '+x.current+'/'+x.total+' · '+x.downloaded+' nuevas · '+x.errors+' errores'):('Descargar todos los episodios MP4 al NAS'+(x.finished?' · última descarga: '+x.downloaded+' nuevas, '+x.skipped+' ya existentes, '+x.errors+' errores':''));if(x.running)setTimeout(poll,2000)}).catch(function(){d.disabled=false;d.classList.remove('mirror-downloading')})}
      d.addEventListener('click',async function(){if(d.disabled)return;if(!confirm('Descargar al NAS todos los episodios MP4 que falten de esta serie?'))return;d.disabled=true;d.classList.add('mirror-downloading');var f=new URLSearchParams();f.set('slug',m[1]);try{var r=await fetch('/api/download-series',{method:'POST',headers:{'Content-Type':'application/x-www-form-urlencoded'},body:f.toString()});if(!r.ok)throw new Error(await r.text());poll()}catch(e){d.disabled=false;d.classList.remove('mirror-downloading');alert('No se pudo iniciar la descarga: '+e.message)}});anchor.insertAdjacentElement('afterend',d);poll()
    }
  }catch(e){console.error(e)}
}
function run(){scheduled=false;fixAttrs(document);ensureActions()}
function schedule(){if(scheduled)return;scheduled=true;requestAnimationFrame(run)}
function start(){run();new MutationObserver(schedule).observe(document.documentElement,{subtree:true,childList:true,attributes:true,attributeFilter:['src','srcset','poster','data-src','data-lazy-src']})}
if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',start,{once:true});else start();
})();</script><style>#mirror-refresh-series,#mirror-download-series{display:inline-flex;align-items:center;justify-content:center}.mirror-refreshing svg,.mirror-downloading svg{animation:mirror-spin .8s linear infinite}@keyframes mirror-spin{to{transform:rotate(360deg)}}</style>`
