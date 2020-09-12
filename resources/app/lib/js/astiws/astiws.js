if (typeof asticode === "undefined") {
    var asticode = {};
}
asticode.ws = {
    init: function(options) {
        const self = this

        if (!self.windowUnloadHandled) {
            window.onbeforeunload = function() {
                self.showOfflineMessage = false;
                if (typeof self.s !== "undefined") {
                    self.s.close();
                }
            }
            self.windowUnloadHandled = true
        }

        const okRequest = options.okRequest
        okRequest.error = function() {
            setTimeout(function() {
                self.init(options)
            }, 1000)
        }
        okRequest.success = function() {
            // Init websocket
            const query = Object.assign({}, options.query)
            self.s = new WebSocket(options.url + "?" + Object.keys(query).map(k => encodeURIComponent(k) + '=' + encodeURIComponent(query[k])).join('&'))

            // Declare functions
            let intervalPing
            self.s.onclose = function() {
                if (self.showOfflineMessage) {
                    self.showOfflineMessage = false
                    if (typeof options.offline !== "undefined") options.offline()
                }
                clearInterval(intervalPing)
                setTimeout(function() {
                    self.init(options)
                }, 1000)
            }
            self.s.onopen = function() {
                self.showOfflineMessage = true
                if (typeof options.pingPeriod !== "undefined") {
                    let pingFunc = options.pingFunc
                    if (typeof pingFunc === "undefined") {
                        pingFunc = function(self) { self.send("ping") }
                    }
                    intervalPing = setInterval(function() { pingFunc(self) }, options.pingPeriod / 1e6)
                }
                if (typeof options.open !== "undefined") options.open()
            }
            self.s.onmessage = function(event) {
                let data = JSON.parse(event.data)
                if (typeof options.message !== "undefined") options.message(data.event_name, data.payload)
                if (typeof options.messageRaw !== "undefined") options.messageRaw(data)
            }
        }

        asticode.tools.sendHttp(okRequest)
    },
    send: function(event_name, payload) {
        this.sendJSON({event_name: event_name, payload: payload})
    },
    sendJSON: function(data) {
        this.s.send(JSON.stringify(data))
    }
}