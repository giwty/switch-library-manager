if (typeof asticode === "undefined") {
    var asticode = {}
}
asticode.progresser = {
    scriptDir: document.currentScript.src.match(/.*\//),
    shouldReset: false,
    new: function(options) {
        return {
            root: options.root,
            steps: {},
            reset: function() {
                this.steps = {}
                this.root.innerHTML = ""
            },
            build: function(progress) {
                // Reset
                this.reset()

                // Create wrapper
                this.wrapper = document.createElement("div")
                this.wrapper.className = "astiprogresser"
                this.root.appendChild(this.wrapper)

                // Loop through steps
                for (let idx = 0; idx <= progress.steps.length; idx++) {
                    // Create step
                    let s = {}

                    // Create circle cell
                    s.circle = document.createElement("div")
                    s.circle.className = "astiprogresser-circle-cell disabled"
                    this.wrapper.appendChild(s.circle)

                    // Create label
                    let l = document.createElement("div")
                    l.className = "astiprogresser-label"
                    s.circle.appendChild(l)

                    // Create circle wrapper
                    let cw = document.createElement("div")
                    cw.className = "astiprogresser-circle-wrapper"
                    s.circle.appendChild(cw)

                    // Create circle bar
                    let cb = document.createElement("div")
                    cb.className = "astiprogresser-circle-bar"
                    cw.appendChild(cb)

                    // Create circle
                    let c = document.createElement("div")
                    c.className = "astiprogresser-circle"
                    cw.appendChild(c)

                    // Create circle check
                    let cc = document.createElement("img")
                    cc.className = "astiprogresser-circle-check"
                    cc.src = asticode.progresser.scriptDir + "/check.png"
                    cw.appendChild(cc)

                    // Create bar
                    if (idx !== progress.steps.length) {
                        // Create bar cell
                        s.barCell = document.createElement("div")
                        s.barCell.className = "astiprogresser-bar-cell disabled"
                        this.wrapper.appendChild(s.barCell)

                        // Create label
                        let l = document.createElement("div")
                        l.className = "astiprogresser-label"
                        l.innerText = progress.steps[idx]
                        s.barCell.appendChild(l)

                        // Create bar wrapper
                        let bw = document.createElement("div")
                        bw.className = "astiprogresser-bar-wrapper"
                        s.barCell.appendChild(bw)

                        // Create bar
                        s.bar = document.createElement("div")
                        s.bar.className = "astiprogresser-bar"
                        bw.appendChild(s.bar)
                    }

                    // Append step
                    this.steps[(idx < progress.steps.length ? progress.steps[idx] : "")] = s
                }
            },
            update: function(progress) {
                // Reset
                if (this.shouldReset) {
                    // Reset
                    this.reset()

                    // Cancel reset
                    this.shouldReset = false
                }

                // Build the progresser
                if (typeof this.wrapper === "undefined") this.build(progress)

                // Update wrapper
                if (typeof this.wrapper !== "undefined") this.wrapper.className = "astiprogresser"

                // Check error
                if (typeof progress.error !== "undefined" && progress.error !== "") {
                    // Update wrapper
                    this.wrapper.className = "astiprogresser error"

                    // Custom
                    if (typeof options.error !== "undefined") options.error(progress.error)

                    // Schedule reset
                    this.shouldReset = true
                }

                // Loop through steps
                let reset = false
                for (let idx = 0; idx < progress.steps.length; idx++) {
                    // Get step
                    let step = progress.steps[idx]

                    // Step doesn't exist, we need to build the progresser
                    if (typeof this.steps[step] === "undefined") {
                        this.build(progress)
                    }

                    // Reset
                    if (reset) {
                        // Update bar
                        this.steps[step].barCell.className = "astiprogresser-bar-cell disabled"
                        this.steps[step].bar.style.width = "0"

                        // Update circle
                        this.steps[step].circle.className = "astiprogresser-circle-cell disabled"
                        continue
                    }

                    // This is the current step
                    if (progress.current_step === step) {
                        // Update reset
                        reset = true

                        // Update bar
                        this.steps[step].bar.style.width = progress.progress + "%"

                        // We've reached the end
                        if (idx === progress.steps.length - 1 && progress.progress === 100) {
                            // Update bar
                            this.steps[step].barCell.className = "astiprogresser-bar-cell done"

                            // Update circle
                            this.steps[step].circle.className = "astiprogresser-circle-cell done"

                            // Update last circle
                            this.steps[""].circle.className = "astiprogresser-circle-cell done"

                            // Schedule reset
                            this.shouldReset = true
                        } else if (idx === progress.steps.length - 1) {
                            // Update bar
                            this.steps[step].barCell.className = "astiprogresser-bar-cell enabled"

                            // Update circle
                            this.steps[step].circle.className = "astiprogresser-circle-cell enabled"

                            // Update last circle
                            this.steps[""].circle.className = "astiprogresser-circle-cell disabled"
                        } else {
                            // Update bar
                            this.steps[step].barCell.className = "astiprogresser-bar-cell enabled"

                            // Update circle
                            this.steps[step].circle.className = "astiprogresser-circle-cell enabled"

                            // Update last circle
                            this.steps[""].circle.className = "astiprogresser-circle-cell disabled"
                        }
                        continue
                    }
                    // Update bar
                    this.steps[step].barCell.className = "astiprogresser-bar-cell done"
                    this.steps[step].bar.style.width = "100%"

                    // Update circle
                    this.steps[step].circle.className = "astiprogresser-circle-cell done"
                }
            },
        }
    }
}