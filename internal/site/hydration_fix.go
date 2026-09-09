package site

// hydrationFixUI mantiene las URLs de imágenes del CDN pasando por el mirror
// incluso después de que SvelteKit hidrate la página y restaure URLs absolutas.
// También vuelve a insertar el botón de refresco si la hidratación lo elimina.
const hydrationFixUI = `<script>(function(){
var CDN='https://cdn.animeav1.com/';
var scheduled=false;
function localizeValue(v){
  if(!v||v.indexOf(CDN)===-1)return v;
  return v.split(CDN).join('/_cdn/');
}
function fixAttrs(root){
  var nodes=[];
  if(root&&root.nodeType===1)nodes.push(root);
  if(root&&root.querySelectorAll)nodes=nodes.concat(Array.prototype.slice.call(root.querySelectorAll('[src],[srcset],[poster],[data-src],[data-lazy-src]')));
  nodes.forEach(function(el){
    ['src','srcset','poster','data-src','data-lazy-src'].forEach(function(a){
      var v=el.getAttribute&&el.getAttribute(a);
      var n=localizeValue(v);
      if(v&&n!==v)el.setAttribute(a,n);
    });
  });
}
function ensureRefresh(){
  try{
    if(!/^\/media\/[^/]+\/?$/.test(location.pathname)||document.getElementById('mirror-refresh-series'))return;
    var nodes=Array.prototype.slice.call(document.querySelectorAll('button,a'));
    var share=nodes.find(function(el){
      var t=(el.textContent||'').trim();
      var a=(el.getAttribute('aria-label')||'')+' '+(el.getAttribute('title')||'');
      return /compartir/i.test(t)||/compartir|share/i.test(a);
    });
    if(!share)return;
    var b=document.createElement('button');
    b.id='mirror-refresh-series';b.type='button';b.className=share.className;
    b.title='Actualizar esta ficha desde AnimeAV1';b.setAttribute('aria-label','Actualizar esta ficha desde AnimeAV1');
    b.innerHTML='<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M20 11a8.1 8.1 0 0 0-15.5-2M4 4v5h5"/><path d="M4 13a8.1 8.1 0 0 0 15.5 2M20 20v-5h-5"/></svg>';
    b.style.marginLeft='4px';
    b.addEventListener('click',async function(){
      if(b.disabled)return;b.disabled=true;b.classList.add('mirror-refreshing');
      try{var r=await fetch('/__mirror/refresh?path='+encodeURIComponent(location.pathname),{method:'POST',headers:{'X-Mirror-Action':'refresh-series'}});if(!r.ok)throw new Error(await r.text());location.reload()}
      catch(e){console.error(e);b.disabled=false;b.classList.remove('mirror-refreshing');alert('No se pudo actualizar la ficha: '+e.message)}
    });
    share.insertAdjacentElement('afterend',b);
  }catch(e){console.error(e)}
}
function run(){scheduled=false;fixAttrs(document);ensureRefresh()}
function schedule(){if(scheduled)return;scheduled=true;requestAnimationFrame(run)}
function start(){run();new MutationObserver(schedule).observe(document.documentElement,{subtree:true,childList:true,attributes:true,attributeFilter:['src','srcset','poster','data-src','data-lazy-src']})}
if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',start,{once:true});else start();
})();</script><style>#mirror-refresh-series{display:inline-flex;align-items:center;justify-content:center}.mirror-refreshing svg{animation:mirror-spin .8s linear infinite}@keyframes mirror-spin{to{transform:rotate(360deg)}}</style>`
