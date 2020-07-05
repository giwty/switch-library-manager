const {dialog} = require('electron').remote;


$(function () {

    let state = {
        settings:{}
    };

    //handle tabs action
    $('.tabgroup > div').hide();
    // loadTab($('.tabgroup > div:first-of-type'));

    // This will wait for the astilectron namespace to be ready
    document.addEventListener('astilectron-ready', function () {
        let sendMessage = function (name, payload, callback) {
            astilectron.sendMessage({name: name, payload: payload}, callback)
        };

        sendMessage("loadSettings", "", function (message) {
            state.settings = JSON.parse(message);
        });

        $(".progress-container").show();
        $(".progress-type").text("Downloading latest Switch titles/versions ...");

        sendMessage("updateDB", "", function (message) {
            scanLocalFolder();
        });

        astilectron.onMessage(function (message) {
            // Process message
            // console.log(message)
            if (message.name === "updateProgress") {
                let pp = JSON.parse(message.payload);
                let count = pp.curr;
                let total = pp.total;
                let pcg = Math.floor(count / total * 100);
                $('.progress-bar').attr('aria-valuenow', pcg);
                $('.progress-bar').attr('style', 'width:' + Number(pcg) + '%');
                $('.progress-bar').text(pcg + "%");
                $('.progress-msg').text(pp.message + " ...");
                if (pcg === 100){
                    $(".progress-container").hide();
                }
                return
            }
            else if (message.name === "libraryLoaded") {
                state.library = JSON.parse(message.payload);
                loadTab("#library")
            }
            else if (message.name === "error") {
                dialog.showMessageBox(null, {
                    type: 'error',
                    buttons: ['Ok'],
                    defaultId: 0,
                    title: 'Error',
                    message: 'An unexpected error occurred',
                    detail: message.payload
                });
                state.settings.folder = undefined;
                $(".progress-container").hide();
                loadTab("#library")
            }
        });

        let openFolderPicker = function () {
            //show info
            dialog.showOpenDialog({
                properties: ['openDirectory'],
                message:"Select games folder"
            }).then(updateFolder)
                .catch(error => console.log(error))
        };

        let scanLocalFolder = function(){
            if (!state.settings.folder){
                loadTab("#library")
                return
            }
            //show progress
            $(".progress-container").show();
            $(".progress-type").text("Scanning local library...");

            sendMessage("updateLocalLibrary", "", (r => {}))
        };

        let updateFolder = function (result) {
            if (result.canceled) {
                console.log("user aborted");
                return
            }
            if (!result.filePaths || !result.filePaths.length){
                return
            }
            state.library = undefined;
            state.updates = undefined;
            state.dlc = undefined;
            $('.tabgroup > div').hide();
            console.log("selected folder:"+result.filePaths[0])
            state.settings.folder = result.filePaths[0]
            sendMessage("saveSettings", JSON.stringify(state.settings), scanLocalFolder);
        };

        function loadTab(target) {
            $(target).show();
            if (target === "#settings") {
                let settingsJSON = JSON.stringify(state.settings, null, 2)
                let settingsHtml = $(target + "Template").render({code: settingsJSON})
                $(target).html(settingsHtml);
                //  asticode.loader.hide()
            } else if (target === "#organize") {
                let html = $(target + "Template").render({folder: state.settings.folder,settings:state.settings})
                $(target).html(html);
            } else if (target === "#updates") {
                if (state.settings.folder && !state.library){
                    return
                }
                if (state.library && !state.updates){
                    sendMessage("missingUpdates", "", (r => {
                        state.updates = JSON.parse(r)
                        loadTab("#updates")
                    }));
                    return
                }
                let html = $(target + "Template").render({folder: state.settings.folder,updates:state.updates})
                $(target).html(html);
                if (state.updates && state.updates.length) {
                    let table = new Tabulator("#updates-table", {
                        layout:"fitDataStretch",
                        pagination: "local",
                        paginationSize: 100,
                        data: state.updates,
                        columns: [
                            {formatter:"rownum"},
                            {field: "Attributes.bannerUrl",formatter:"image", headerSort:false,formatterParams:{height:"60px", width:"60px"}},
                            {title: "Title", field: "Attributes.name", headerFilter:"input"},
                            {title: "Type", field: "Meta.type", headerFilter:"input"},
                            {title: "Title id", headerSort:false, field: "Attributes.id", hozAlign: "right", sorter: "number"},
                            {title: "Local version", headerSort:false, field: "local_update", hozAlign: "right", sorter: "number"},
                            {title: "Available version", headerSort:false, field: "latest_update", hozAlign: "right"},
                            {title: "Update date", headerSort:false, field: "latest_update_date", widthGrow: 2}
                        ],
                    });
                }
            } else if (target === "#dlc") {
                if (state.settings.folder && !state.library){
                    return
                }
                if (state.library && !state.dlc){
                    sendMessage("missingDlc", "", (r => {
                        state.dlc = JSON.parse(r)
                        loadTab("#dlc")
                    }));
                    return
                }
                let html = $(target + "Template").render({folder: state.settings.folder,dlc:state.dlc});
                $(target).html(html);
                if (state.dlc && state.dlc.length) {
                    let table = new Tabulator("#dlc-table", {
                        layout:"fitDataStretch",
                        pagination: "local",
                        paginationSize: 100,
                        data: state.dlc,
                        columns: [
                            {formatter:"rownum"},
                            {field: "Attributes.bannerUrl",formatter:"image", headerSort:false,formatterParams:{height:"60px", width:"60px"}},
                            {title: "Title", field: "Attributes.name", headerFilter:"input"},
                            {title: "# Missing", field: "missing_dlc.length"},
                            {title: "Missing DLC", headerSort:false, field: "missing_dlc",formatter:function(cell, formatterParams, onRendered){
                                    value = ""
                                    for (var i in cell.getValue())
                                    {
                                        value +="<div>"+cell.getValue()[i]+"</div>"
                                    }
                                    return value
                                }}
                        ],
                    });
                }
            } else if (target === "#library") {
                if (state.settings.folder && !state.library){
                    return
                }
                let html = $(target + "Template").render({folder: state.settings.folder,library:state.library})
                $(target).html(html);
                if (state.library && state.library.length) {
                    var table = new Tabulator("#library-table", {
                        layout:"fitDataStretch",
                        pagination: "local",
                        paginationSize: 100,
                        data: state.library,
                        columns: [
                            {formatter:"rownum"},
                            {field: "icon",formatter:"image", headerSort:false,formatterParams:{height:"60px", width:"60px"}},
                            {title: "Title", field: "name", headerFilter:"input"},
                            {title: "Title id", headerSort:false, field: "titleId", hozAlign: "right", sorter: "number"},
                            {title: "File name", headerSort:false, field: "path", widthGrow: 2}
                        ],
                    });
                }
            }
        }

        $("body").on("click", ".folder-set", e => {
            openFolderPicker()
        });

        $("body").on("click", ".library-organize-action", e => {
            e.preventDefault();
            const options = {
                type: 'warning',
                buttons: ['Yes', 'No'],
                defaultId: 0,
                title: 'Confirmation',
                message: 'Are you sure you want to begin library organization?',
                detail: 'This action will modify your local library files',
            };

            dialog.showMessageBox(null, options).then( (r) => {

                if (r.response === 0) {
                    //show progress
                    $('.tabgroup > div').hide();
                    $(".progress-container").show();
                    $(".progress-type").text("Organizing local library...");

                    sendMessage("organize", "", (r => {
                        $(".progress-container").hide();
                        loadTab("#organize");
                        dialog.showMessageBox(null, {
                            type: 'info',
                            buttons: ['Ok'],
                            defaultId: 0,
                            title: 'Success',
                            message: 'Operation completed successfully'
                        })
                    }))
                }
            });

        });

        $('.tabs a').click(function (e) {
            e.preventDefault();
            let $this = $(e.currentTarget);
            let tabgroup = '#' + $this.parents('.tabs').data('tabgroup');
            let others = $this.closest('li').siblings().children('a');
            let target = $this.attr('href');
            others.removeClass('active');
            $this.addClass('active');
            $(tabgroup).children('div').hide();
            loadTab(target)
        });
    })

});