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

document.addEventListener("click", (event) => {
	const clicked = event.target;
	if (!(clicked instanceof Element)) {
		return;
	}
	const addButton = clicked.closest("[data-add-row]");
	if (addButton instanceof HTMLButtonElement) {
		const template = document.getElementById(addButton.dataset.addRow ?? "");
		const target = document.getElementById(addButton.dataset.rowTarget ?? "");
		if (!(template instanceof HTMLTemplateElement) || target === null) {
			return;
		}
		const row = template.content.cloneNode(true);
		target.append(row);
		const rows = target.querySelectorAll("[data-configuration-row]");
		rows
			.item(rows.length - 1)
			?.querySelector("input, select, textarea")
			?.focus();
		return;
	}
	const removeButton = clicked.closest("[data-remove-row]");
	if (removeButton instanceof HTMLButtonElement) {
		removeButton.closest("[data-configuration-row]")?.remove();
	}
});
