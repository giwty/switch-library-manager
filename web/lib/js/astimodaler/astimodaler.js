if (typeof asticode === "undefined") {
    var asticode = {};
}
asticode.modaler = {
    scriptDir: document.currentScript.src.match(/.*\//),
    close: function() {
        if (typeof asticode.modaler.onclose !== "undefined" && asticode.modaler.onclose !== null) {
            asticode.modaler.onclose();
        }
        asticode.modaler.hide();
    },
    hide: function() {
        document.getElementById("astimodaler").style.display = "none";
    },
    init: function() {
        document.body.innerHTML = `<div class="astimodaler" id="astimodaler">
            <div class="astimodaler-background"></div>
            <div class="astimodaler-table">
                <div class="astimodaler-wrapper">
                    <div id="astimodaler-body">
                        <img class="astimodaler-close" src="` + asticode.modaler.scriptDir + `/cross.png" onclick="asticode.modaler.close()"/>
                        <div id="astimodaler-content"></div>
                    </div>
                </div>
            </div>
        </div>` + document.body.innerHTML;
    },
    setContent: function(content) {
        document.getElementById("astimodaler-content").innerHTML = '';
        if (typeof content.node !== "undefined") content = content.node
        document.getElementById("astimodaler-content").appendChild(content);
    },
    setWidth: function(width) {
        document.getElementById("astimodaler-body").style.width = width;
    },
    show: function() {
        document.getElementById("astimodaler").style.display = "block";
    },
    newForm: function() {
        // Create form
        let f = {
            fields: [],
            node: document.createElement("div"),
            addTitle: function(text) {
                let t = document.createElement("div")
                t.className = "astimodaler-title"
                t.innerText = text
                this.node.appendChild(t)
            },
            addError: function() {
                let e = document.createElement("div")
                e.className = "astimodaler-error"
                this.node.appendChild(e)
                this.error = e
            },
            showError: function(text) {
                this.error.innerText = text
                this.error.style.display = "block"
            },
            hideError: function() {
                this.error.style.display = "none"
            },
            addField: function(options) {
                switch (options.type) {
                    case "submit":
                        // Store success
                        this.success = options.success

                        // Create button
                        let b = document.createElement("div")
                        b.className = "astimodaler-field-submit" + (typeof options.className !== "undefined" ? " " + options.className : "")
                        b.innerText = options.label
                        this.node.appendChild(b)

                        // Handle click
                        b.addEventListener("click", function() {
                            this.submit()
                        }.bind(this))
                        break
                    case "email":
                    case "select":
                    case "text":
                    case "textarea":
                        // Create label
                        let l = document.createElement("div")
                        l.className = "astimodaler-label"
                        l.innerHTML = options.label + (typeof options.required !== "undefined" && options.required ? "<span class='astimodaler-required'>*</span>" : "")
                        this.node.appendChild(l)

                        // Create element
                        let i
                        switch (options.type) {
                            case "email":
                            case "text":
                                i = document.createElement("input")
                                i.className = "astimodaler-field-text"
                                i.type = "text"
                                break
                            case "textarea":
                                i = document.createElement("textarea")
                                i.className = "astimodaler-field-textarea"
                                break
                            case "select":
                                i = document.createElement("select")
                                i.className = "astimodaler-field-select"
                                for (let k in options.values) {
                                    if (Object.prototype.hasOwnProperty.call(options.values, k)) {
                                        let o = document.createElement("option")
                                        o.value = k
                                        o.innerText = options.values[k]
                                        i.appendChild(o)
                                    }
                                }
                                break
                        }

                        // Append field
                        this.node.appendChild(i)
                        this.fields.push({
                            node: i,
                            options: options
                        })
                        break
                }
            },
            submit: function() {
                // Hide error
                this.hideError()

                // Loop through fields
                let fs = {}
                for (let i = 0; i < this.fields.length; i++) {
                    // Get field
                    const f = this.fields[i]

                    // Get value
                    let v
                    switch (f.options.type) {
                        case "email":
                        case "text":
                        case "textarea":
                            v = f.node.value
                            break
                        case "select":
                            v = f.node.options[f.node.selectedIndex].value
                            break
                    }

                    // Check required
                    if (typeof f.options.required !== "undefined" && f.options.required && v === "") {
                        this.showError('Field "' + f.options.label + '" is required')
                        return
                    }

                    // Check email
                    if (f.options.type === "email" && !asticode.tools.isEmail(v)) {
                        this.showError(v + " is not a valid email")
                        return
                    }

                    // Append field
                    fs[f.options.name] = v
                }

                // Success callback
                this.success(fs)

            },
            focus: function() {
                // No fields
                if (this.fields.length === 0) { return }

                // Focus first field
                this.fields[0].node.focus()
            },
        }

        // Submit on Enter
        f.node.addEventListener("keyup", function(e) {
            if (e.key === "Enter") {
                f.submit()
            }
        })
        return f
    }
};