
(function(){
	var grid = document.getElementById('masonry-grid');
	if (!grid) return;
	var GAP = 14, ROW = 2;
	function layout() {
		grid.style.rowGap = '0px';
		grid.style.gridAutoRows = ROW + 'px';
		var items = grid.children, heights = [], i;
		for (i = 0; i < items.length; i++) heights.push(items[i].getBoundingClientRect().height);
		for (i = 0; i < items.length; i++) items[i].style.gridRowEnd = 'span ' + Math.max(1, Math.round((heights[i] + GAP) / ROW));
	}
	var pending = false;
	function schedule() {
		if (pending) return;
		pending = true;
		requestAnimationFrame(function(){ pending = false; layout(); });
	}
	layout();
	window.addEventListener('resize', schedule);
	window.addEventListener('load', schedule);
	// Tiles resize when the viewport changes and when a lazy image
	// finally decodes at its true aspect ratio; re-measure either way.
	if (window.ResizeObserver) {
		var observer = new ResizeObserver(schedule);
		for (var n = 0; n < grid.children.length; n++) observer.observe(grid.children[n]);
	}
})();

document.addEventListener('keydown',(e)=>{if(e.key==='Escape'){const m=document.getElementById('photo-modal');if(m)m.innerHTML='';}});document.addEventListener('click', (event) => {
 const close = event.target.closest('[data-close-photo]');
 if (!close || (event.target.closest('[data-photo-content]') && close.tagName !== 'BUTTON')) return;
 const modal = document.getElementById('photo-modal');
 if (modal) modal.replaceChildren();
});
