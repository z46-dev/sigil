const identificationEndpoint = "./api/v1/identify";
const challengeEndpoint = "./api/v1/challenge";
const consentKey = "sigil-fingerprinting-consent-v1";
const acceptConsent = document.querySelector("#accept-consent");
const collectButton = document.querySelector("#collect-button");
const collectedData = document.querySelector("#collected-data");
const consentControl = document.querySelector("#consent-control");
const consentDialog = document.querySelector("#consent-dialog");
const declineConsent = document.querySelector("#decline-consent");
const decision = document.querySelector("#decision");
const identifiers = document.querySelector("#identifiers");
const snapshotID = document.querySelector("#snapshot-id");
const statusElement = document.querySelector("#status");
const visitorID = document.querySelector("#visitor-id");

const showStatus = (message, isError = false) => {
    statusElement.textContent = message;
    statusElement.classList.toggle("error", isError);
};

const showJSON = (value) => {
    const json = JSON.stringify(value, null, 2);

    if (window.hljs) {
        collectedData.innerHTML = window.hljs.highlight(json, { language: "json" }).value;
    } else {
        collectedData.textContent = json;
    }
};

const Go = window.Go || new Object();
const loadCollector = async () => {
    const go = new Go();
    const response = await fetch("./main.wasm");

    if (!response.ok) {
        throw new Error(`Unable to load the browser collector (${response.status}).`);
    }

    const result = await WebAssembly.instantiateStreaming(response, go.importObject);
    void go.run(result.instance);

    if (!window.sigil?.collect) {
        throw new Error("The browser collector did not start.");
    }
};

const identify = async () => {
    collectButton.disabled = true;
    identifiers.hidden = true;
    showStatus("Collecting browser signals…");

    try {
        const challengeResponse = await fetch(challengeEndpoint, { cache: "no-store" });
        const challenge = await challengeResponse.json();

        if (!challengeResponse.ok) {
            throw new Error(challenge.error || `Unable to obtain a challenge (${challengeResponse.status}).`);
        }

        const snapshot = await window.sigil.collect({ mode: "device" });
        const response = await fetch(identificationEndpoint, {
            method: "POST",
            headers: {
                "Content-Type": "application/json",
            },
            body: JSON.stringify({ challenge: challenge.challenge, snapshot }),
        });
        const match = await response.json();

        if (!response.ok) {
            throw new Error(match.error || `Identification failed (${response.status}).`);
        }

        visitorID.textContent = match.visitorId || "Not assigned";
        snapshotID.textContent = snapshot.snapshotId;
        decision.textContent = match.decision;
        showJSON({ snapshot, match });
        identifiers.hidden = false;
        showStatus("Fingerprint collected successfully.");
    } catch (error) {
        collectedData.textContent = error instanceof Error ? error.stack || error.message : String(error);
        showStatus(error instanceof Error ? error.message : "Fingerprint collection failed.", true);
    } finally {
        collectButton.disabled = false;
    }
};

collectButton.addEventListener("click", identify);

const start = async () => {
    try {
        await loadCollector();
        await identify();
    } catch (error) {
        collectedData.textContent = error instanceof Error ? error.stack || error.message : String(error);
        showStatus(error instanceof Error ? error.message : "Unable to start Sigil.", true);
    }
};

acceptConsent.addEventListener("click", () => {
    localStorage.setItem(consentKey, "accepted");
    consentDialog.close();
    consentControl.textContent = "Revoke consent";
    void start();
});

declineConsent.addEventListener("click", () => {
    localStorage.setItem(consentKey, "declined");
    consentDialog.close();
    consentControl.textContent = "Review consent choice";
    collectButton.disabled = true;
    collectedData.textContent = "No browser data was collected.";
    showStatus("Fingerprinting declined. No browser data was collected.");
});

consentControl.addEventListener("click", () => {
    if (localStorage.getItem(consentKey) === "accepted") {
        localStorage.setItem(consentKey, "declined");
        window.location.reload();
        return;
    }

    consentDialog.showModal();
});

if (localStorage.getItem(consentKey) === "accepted") {
    await start();
} else if (localStorage.getItem(consentKey) === "declined") {
    collectButton.disabled = true;
    consentControl.textContent = "Review consent choice";
    collectedData.textContent = "No browser data was collected.";
    showStatus("Fingerprinting is disabled.");
} else {
    collectButton.disabled = true;
    showStatus("Waiting for your consent before collecting browser data.");
    consentDialog.showModal();
}
