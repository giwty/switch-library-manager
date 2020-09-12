if (typeof asticode === "undefined") {
    var asticode = {};
}
asticode.loader = {
    scriptDir: document.currentScript.src.match(/.*\//),
    hide: function() {
        document.getElementById("astiloader").style.display = "none";
    },
    init: function() {
        document.body.innerHTML = `
        <div class="astiloader" id="astiloader">
            <div class="astiloader-background"></div>
            <div class="astiloader-table"><div class="astiloader-content"><img src="` + asticode.loader.scriptDir + `/loader.png"/></div></div>
        </div>
        ` + document.body.innerHTML
    },
    show: function() {
        document.getElementById("astiloader").style.display = "block";
    }
};