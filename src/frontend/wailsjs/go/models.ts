export namespace app {
	
	export class GameDLCDetail {
	    titleId: string;
	    name: string;
	    attributes: db.TitleAttributes;
	    status: string;
	    update_status: string;
	    local_version: number;
	    has_local_version: boolean;
	    available_version: number;
	    has_available_version: boolean;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new GameDLCDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.titleId = source["titleId"];
	        this.name = source["name"];
	        this.attributes = this.convertValues(source["attributes"], db.TitleAttributes);
	        this.status = source["status"];
	        this.update_status = source["update_status"];
	        this.local_version = source["local_version"];
	        this.has_local_version = source["has_local_version"];
	        this.available_version = source["available_version"];
	        this.has_available_version = source["has_available_version"];
	        this.path = source["path"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class GameUpdateDetail {
	    version: number;
	    date: string;
	    status: string;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new GameUpdateDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.date = source["date"];
	        this.status = source["status"];
	        this.path = source["path"];
	    }
	}
	export class GameDetails {
	    attributes: db.TitleAttributes;
	    titleId: string;
	    name: string;
	    release_date_text: string;
	    remote_available: boolean;
	    metadata_available: boolean;
	    base_collected: boolean;
	    base_path: string;
	    updates: GameUpdateDetail[];
	    local_only_updates: GameUpdateDetail[];
	    dlc: GameDLCDetail[];
	    local_only_dlc: GameDLCDetail[];
	
	    static createFrom(source: any = {}) {
	        return new GameDetails(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.attributes = this.convertValues(source["attributes"], db.TitleAttributes);
	        this.titleId = source["titleId"];
	        this.name = source["name"];
	        this.release_date_text = source["release_date_text"];
	        this.remote_available = source["remote_available"];
	        this.metadata_available = source["metadata_available"];
	        this.base_collected = source["base_collected"];
	        this.base_path = source["base_path"];
	        this.updates = this.convertValues(source["updates"], GameUpdateDetail);
	        this.local_only_updates = this.convertValues(source["local_only_updates"], GameUpdateDetail);
	        this.dlc = this.convertValues(source["dlc"], GameDLCDetail);
	        this.local_only_dlc = this.convertValues(source["local_only_dlc"], GameDLCDetail);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class LibraryTemplateData {
	    id: number;
	    name: string;
	    version: string;
	    dlc: string;
	    titleId: string;
	    path: string;
	    icon: string;
	    update: number;
	    region: string;
	    type: string;
	
	    static createFrom(source: any = {}) {
	        return new LibraryTemplateData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.version = source["version"];
	        this.dlc = source["dlc"];
	        this.titleId = source["titleId"];
	        this.path = source["path"];
	        this.icon = source["icon"];
	        this.update = source["update"];
	        this.region = source["region"];
	        this.type = source["type"];
	    }
	}
	export class Pair {
	    key: string;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new Pair(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.value = source["value"];
	    }
	}
	export class LocalLibraryData {
	    library_data: LibraryTemplateData[];
	    issues: Pair[];
	    num_files: number;
	
	    static createFrom(source: any = {}) {
	        return new LocalLibraryData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.library_data = this.convertValues(source["library_data"], LibraryTemplateData);
	        this.issues = this.convertValues(source["issues"], Pair);
	        this.num_files = source["num_files"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class OrganizationPreviewEntry {
	    kind: string;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new OrganizationPreviewEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.path = source["path"];
	    }
	}
	export class OrganizationPreview {
	    root_folder: string;
	    entries: OrganizationPreviewEntry[];
	
	    static createFrom(source: any = {}) {
	        return new OrganizationPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.root_folder = source["root_folder"];
	        this.entries = this.convertValues(source["entries"], OrganizationPreviewEntry);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class OrganizationResult {
	    status: string;
	    conflict_count: number;
	    library: LocalLibraryData;
	
	    static createFrom(source: any = {}) {
	        return new OrganizationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.conflict_count = source["conflict_count"];
	        this.library = this.convertValues(source["library"], LocalLibraryData);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class SwitchTitle {
	    name: string;
	    titleId: string;
	    icon: string;
	    region: string;
	    release_date: string;
	
	    static createFrom(source: any = {}) {
	        return new SwitchTitle(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.titleId = source["titleId"];
	        this.icon = source["icon"];
	        this.region = source["region"];
	        this.release_date = source["release_date"];
	    }
	}

}

export namespace db {
	
	export class TitleAttributes {
	    id: string;
	    name?: string;
	    version?: string;
	    region?: string;
	    releaseDate?: number;
	    ParsedReleaseDate: string;
	    publisher?: string;
	    iconUrl?: string;
	    screenshots?: string[];
	    bannerUrl?: string;
	    description?: string;
	    size?: number;
	    isDemo?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TitleAttributes(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.version = source["version"];
	        this.region = source["region"];
	        this.releaseDate = source["releaseDate"];
	        this.ParsedReleaseDate = source["ParsedReleaseDate"];
	        this.publisher = source["publisher"];
	        this.iconUrl = source["iconUrl"];
	        this.screenshots = source["screenshots"];
	        this.bannerUrl = source["bannerUrl"];
	        this.description = source["description"];
	        this.size = source["size"];
	        this.isDemo = source["isDemo"];
	    }
	}

}

export namespace process {
	
	export class MissingDLC {
	    id: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new MissingDLC(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	    }
	}
	export class IncompleteTitle {
	    Attributes: db.TitleAttributes;
	    Meta?: switchfs.ContentMetaAttributes;
	    local_update: number;
	    local_display_version: string;
	    latest_update: number;
	    latest_update_date: string;
	    missing_dlc: MissingDLC[];
	
	    static createFrom(source: any = {}) {
	        return new IncompleteTitle(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Attributes = this.convertValues(source["Attributes"], db.TitleAttributes);
	        this.Meta = this.convertValues(source["Meta"], switchfs.ContentMetaAttributes);
	        this.local_update = source["local_update"];
	        this.local_display_version = source["local_display_version"];
	        this.latest_update = source["latest_update"];
	        this.latest_update_date = source["latest_update_date"];
	        this.missing_dlc = this.convertValues(source["missing_dlc"], MissingDLC);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace reportexport {
	
	export class Report {
	    kind: string;
	    headers: string[];
	    rows: any[][];
	
	    static createFrom(source: any = {}) {
	        return new Report(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.headers = source["headers"];
	        this.rows = source["rows"];
	    }
	}

}

export namespace settings {
	
	export class LoggingSettings {
	    debug: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LoggingSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.debug = source["debug"];
	    }
	}
	export class DataSourceSettings {
	    titles_url: string;
	    versions_url: string;
	
	    static createFrom(source: any = {}) {
	        return new DataSourceSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.titles_url = source["titles_url"];
	        this.versions_url = source["versions_url"];
	    }
	}
	export class MissingContentSettings {
	    check_for_updates: boolean;
	    check_for_dlc: boolean;
	    ignore_dlc_updates: boolean;
	    ignore_dlc_title_ids: string[];
	    ignore_update_title_ids: string[];
	
	    static createFrom(source: any = {}) {
	        return new MissingContentSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.check_for_updates = source["check_for_updates"];
	        this.check_for_dlc = source["check_for_dlc"];
	        this.ignore_dlc_updates = source["ignore_dlc_updates"];
	        this.ignore_dlc_title_ids = source["ignore_dlc_title_ids"];
	        this.ignore_update_title_ids = source["ignore_update_title_ids"];
	    }
	}
	export class OrganizationSettings {
	    create_folder_per_game: boolean;
	    move_scan_files_to_library: boolean;
	    dlc_folder: string;
	    updates_folder: string;
	    rename_files: boolean;
	    delete_empty_folders: boolean;
	    delete_old_update_files: boolean;
	    folder_name_template: string;
	    allow_unicode_file_names: boolean;
	    file_name_template: string;
	    include_base_type: boolean;
	    process_when_missing_base_game: boolean;
	
	    static createFrom(source: any = {}) {
	        return new OrganizationSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.create_folder_per_game = source["create_folder_per_game"];
	        this.move_scan_files_to_library = source["move_scan_files_to_library"];
	        this.dlc_folder = source["dlc_folder"];
	        this.updates_folder = source["updates_folder"];
	        this.rename_files = source["rename_files"];
	        this.delete_empty_folders = source["delete_empty_folders"];
	        this.delete_old_update_files = source["delete_old_update_files"];
	        this.folder_name_template = source["folder_name_template"];
	        this.allow_unicode_file_names = source["allow_unicode_file_names"];
	        this.file_name_template = source["file_name_template"];
	        this.include_base_type = source["include_base_type"];
	        this.process_when_missing_base_game = source["process_when_missing_base_game"];
	    }
	}
	export class ScanSettings {
	    recursive: boolean;
	    rescan_on_startup: boolean;
	    ignore_file_types: string[];
	    ignore_unsupported_file_types: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ScanSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.recursive = source["recursive"];
	        this.rescan_on_startup = source["rescan_on_startup"];
	        this.ignore_file_types = source["ignore_file_types"];
	        this.ignore_unsupported_file_types = source["ignore_unsupported_file_types"];
	    }
	}
	export class PathSettings {
	    library_folder: string;
	    scan_folders: string[];
	    prod_keys: string;
	
	    static createFrom(source: any = {}) {
	        return new PathSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.library_folder = source["library_folder"];
	        this.scan_folders = source["scan_folders"];
	        this.prod_keys = source["prod_keys"];
	    }
	}
	export class GUISettings {
	    enabled: boolean;
	    page_size: number;
	    hide_missing_games: boolean;
	    hide_demo_games: boolean;
	    hide_version_string: boolean;
	    remember_window_state: boolean;
	    theme: string;
	
	    static createFrom(source: any = {}) {
	        return new GUISettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.page_size = source["page_size"];
	        this.hide_missing_games = source["hide_missing_games"];
	        this.hide_demo_games = source["hide_demo_games"];
	        this.hide_version_string = source["hide_version_string"];
	        this.remember_window_state = source["remember_window_state"];
	        this.theme = source["theme"];
	    }
	}
	export class AppSettings {
	    schema_version: number;
	    gui: GUISettings;
	    paths: PathSettings;
	    scan: ScanSettings;
	    organization: OrganizationSettings;
	    missing_content: MissingContentSettings;
	    data_sources: DataSourceSettings;
	    logging: LoggingSettings;
	
	    static createFrom(source: any = {}) {
	        return new AppSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema_version = source["schema_version"];
	        this.gui = this.convertValues(source["gui"], GUISettings);
	        this.paths = this.convertValues(source["paths"], PathSettings);
	        this.scan = this.convertValues(source["scan"], ScanSettings);
	        this.organization = this.convertValues(source["organization"], OrganizationSettings);
	        this.missing_content = this.convertValues(source["missing_content"], MissingContentSettings);
	        this.data_sources = this.convertValues(source["data_sources"], DataSourceSettings);
	        this.logging = this.convertValues(source["logging"], LoggingSettings);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	
	
	

}

export namespace switchfs {
	
	export class Content {
	    Text: string;
	    Type: string;
	    ID: string;
	    Size: string;
	    Hash: string;
	    KeyGeneration: string;
	
	    static createFrom(source: any = {}) {
	        return new Content(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Text = source["Text"];
	        this.Type = source["Type"];
	        this.ID = source["ID"];
	        this.Size = source["Size"];
	        this.Hash = source["Hash"];
	        this.KeyGeneration = source["KeyGeneration"];
	    }
	}
	export class NacpTitle {
	    Language: number;
	    Title: string;
	
	    static createFrom(source: any = {}) {
	        return new NacpTitle(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Language = source["Language"];
	        this.Title = source["Title"];
	    }
	}
	export class Nacp {
	    TitleName: Record<string, NacpTitle>;
	    Isbn: string;
	    DisplayVersion: string;
	    SupportedLanguageFlag: number;
	
	    static createFrom(source: any = {}) {
	        return new Nacp(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.TitleName = this.convertValues(source["TitleName"], NacpTitle, true);
	        this.Isbn = source["Isbn"];
	        this.DisplayVersion = source["DisplayVersion"];
	        this.SupportedLanguageFlag = source["SupportedLanguageFlag"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ContentMetaAttributes {
	    title_id: string;
	    version: number;
	    type: string;
	    Contents: Record<string, Content>;
	    Ncap?: Nacp;
	
	    static createFrom(source: any = {}) {
	        return new ContentMetaAttributes(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title_id = source["title_id"];
	        this.version = source["version"];
	        this.type = source["type"];
	        this.Contents = this.convertValues(source["Contents"], Content, true);
	        this.Ncap = this.convertValues(source["Ncap"], Nacp);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	

}

