document.addEventListener("keydown", (event) => {
	if (event.key !== "Tab" || event.altKey || event.ctrlKey || event.metaKey) {
		return;
	}
	const target = event.target;
	if (!(target instanceof HTMLTextAreaElement)) {
		return;
	}
	if (target.dataset.tabInsertsSpaces !== "true") {
		return;
	}
	event.preventDefault();
	const insert = "    ";
	const start = target.selectionStart;
	const end = target.selectionEnd;
	target.setRangeText(insert, start, end, "end");
});
