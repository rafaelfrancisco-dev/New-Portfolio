
(() => {
	const form = document.getElementById("post-editor");
	const body = document.getElementById("body_md");
	if (!form || !body) return;
let dirty = false;
	form.addEventListener("input", () => dirty = true);
	form.addEventListener("submit", () => dirty = false);
	document.querySelectorAll(".media-insert").forEach((button) => button.addEventListener("click", () => {
		const start = body.selectionStart;
		body.setRangeText(button.dataset.markdown, start, body.selectionEnd, "end");
		body.dispatchEvent(new Event("input", { bubbles: true }));
	}));
	window.addEventListener("beforeunload", (event) => {
		if (!dirty) return;
		event.preventDefault();
		event.returnValue = "";
	});
})();
document.addEventListener('submit', (event) => {
 const message = event.target.dataset.confirm;
 if (message && !window.confirm(message)) event.preventDefault();
});
