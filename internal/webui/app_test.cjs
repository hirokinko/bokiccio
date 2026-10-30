const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

const applicationScript = fs.readFileSync(
	path.join(__dirname, "assets", "app.js"),
	"utf8",
);

function loadApplication(href) {
	const listeners = new Map();
	const assigned = [];
	const replaced = [];
	const location = { href, assign: (target) => assigned.push(target) };
	const history = {
		state: { test: true },
		replaceState: (state, _unused, target) => {
			replaced.push({ state, target });
			location.href = new URL(target, location.href).href;
		},
	};
	const document = {
		addEventListener: (name, listener) => {
			const registered = listeners.get(name) ?? [];
			registered.push(listener);
			listeners.set(name, registered);
		},
	};
	class Element {}
	class HTMLButtonElement extends Element {}
	class HTMLTemplateElement extends Element {}
	class HTMLTextAreaElement extends Element {}
	vm.runInNewContext(applicationScript, {
		document,
		Element,
		HTMLButtonElement,
		HTMLTemplateElement,
		HTMLTextAreaElement,
		URL,
		window: { history, location },
	});
	return {
		assigned,
		dispatch: (name, detail) => {
			for (const listener of listeners.get(name) ?? []) {
				listener({ detail });
			}
		},
		href: () => location.href,
		replaced,
	};
}

test("removes the IAP authentication mode while preserving application URL state", () => {
	const application = loadApplication(
		"https://example.test/en/reports/current?as_of=2026-10-30&gcp-iap-mode=AUTHENTICATING#balances",
	);
	assert.deepEqual(application.replaced, [
		{
			state: { test: true },
			target: "/en/reports/current?as_of=2026-10-30#balances",
		},
	]);
	assert.equal(
		application.href(),
		"https://example.test/en/reports/current?as_of=2026-10-30#balances",
	);
});

test("leaves other IAP modes unchanged", () => {
	const application = loadApplication(
		"https://example.test/?gcp-iap-mode=IDENTITY",
	);
	assert.deepEqual(application.replaced, []);
	application.dispatch("htmx:responseError", { xhr: { status: 401 } });
	assert.deepEqual(application.assigned, ["/?gcp-iap-mode=IDENTITY"]);
});

test("marks htmx requests as XMLHttpRequest", () => {
	const application = loadApplication("https://example.test/");
	const headers = {};
	application.dispatch("htmx:configRequest", { headers });
	assert.equal(headers["X-Requested-With"], "XMLHttpRequest");
});

test("recovers once from HTTP 401 using a clean same-origin application URL", () => {
	const application = loadApplication(
		"https://example.test/en/reports/current?as_of=2026-10-30&gcp-iap-mode=AUTHENTICATING#balances",
	);
	application.dispatch("htmx:responseError", { xhr: { status: 401 } });
	application.dispatch("htmx:responseError", { xhr: { status: 401 } });
	assert.deepEqual(application.assigned, [
		"/en/reports/current?as_of=2026-10-30#balances",
	]);
});

test("does not treat other response errors as authentication failures", () => {
	for (const status of [400, 403, 500]) {
		const application = loadApplication("https://example.test/");
		application.dispatch("htmx:responseError", { xhr: { status } });
		assert.deepEqual(application.assigned, []);
	}
});
