export namespace apperrors {
	
	export class PublicError {
	    code: string;
	    message: string;
	    details?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new PublicError(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.message = source["message"];
	        this.details = source["details"];
	    }
	}

}

export namespace domain {
	
	export class OutputFile {
	    path: string;
	    durationSeconds: number;
	
	    static createFrom(source: any = {}) {
	        return new OutputFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.durationSeconds = source["durationSeconds"];
	    }
	}
	export class CutResult {
	    jobId: string;
	    outputDirectory: string;
	    perSourceOutputs: OutputFile[];
	    mergedOutput?: OutputFile;
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new CutResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.jobId = source["jobId"];
	        this.outputDirectory = source["outputDirectory"];
	        this.perSourceOutputs = this.convertValues(source["perSourceOutputs"], OutputFile);
	        this.mergedOutput = this.convertValues(source["mergedOutput"], OutputFile);
	        this.warnings = source["warnings"];
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

export namespace jobs {
	
	export class DetectionSnapshot {
	    jobId: string;
	    workspaceId: number;
	    status: string;
	    videosTotal: number;
	    videosDone: number;
	    currentVideo: string;
	    tracksFound: number;
	    error?: apperrors.PublicError;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	    // Go type: time
	    finishedAt?: any;
	
	    static createFrom(source: any = {}) {
	        return new DetectionSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.jobId = source["jobId"];
	        this.workspaceId = source["workspaceId"];
	        this.status = source["status"];
	        this.videosTotal = source["videosTotal"];
	        this.videosDone = source["videosDone"];
	        this.currentVideo = source["currentVideo"];
	        this.tracksFound = source["tracksFound"];
	        this.error = this.convertValues(source["error"], apperrors.PublicError);
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.finishedAt = this.convertValues(source["finishedAt"], null);
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
	export class Snapshot {
	    jobId: string;
	    status: string;
	    totalSegments: number;
	    completedSegments: number;
	    currentFile: string;
	    result?: domain.CutResult;
	    error?: apperrors.PublicError;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	    // Go type: time
	    finishedAt?: any;
	
	    static createFrom(source: any = {}) {
	        return new Snapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.jobId = source["jobId"];
	        this.status = source["status"];
	        this.totalSegments = source["totalSegments"];
	        this.completedSegments = source["completedSegments"];
	        this.currentFile = source["currentFile"];
	        this.result = this.convertValues(source["result"], domain.CutResult);
	        this.error = this.convertValues(source["error"], apperrors.PublicError);
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.finishedAt = this.convertValues(source["finishedAt"], null);
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

export namespace wailsapi {
	
	export class ActionResponse {
	    success: boolean;
	    error?: apperrors.PublicError;
	
	    static createFrom(source: any = {}) {
	        return new ActionResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = this.convertValues(source["error"], apperrors.PublicError);
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
	export class CSVImportCutDTO {
	    row: number;
	    name: string;
	    datetimeLocal: string;
	    secondsBefore: number;
	    secondsAfter: number;
	    outputDirectory?: string;
	    totalSegments?: number;
	
	    static createFrom(source: any = {}) {
	        return new CSVImportCutDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.row = source["row"];
	        this.name = source["name"];
	        this.datetimeLocal = source["datetimeLocal"];
	        this.secondsBefore = source["secondsBefore"];
	        this.secondsAfter = source["secondsAfter"];
	        this.outputDirectory = source["outputDirectory"];
	        this.totalSegments = source["totalSegments"];
	    }
	}
	export class CSVImportIssueDTO {
	    row: number;
	    column: string;
	    message: string;
	    value?: string;
	
	    static createFrom(source: any = {}) {
	        return new CSVImportIssueDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.row = source["row"];
	        this.column = source["column"];
	        this.message = source["message"];
	        this.value = source["value"];
	    }
	}
	export class CutJobStatusResponse {
	    job?: jobs.Snapshot;
	    error?: apperrors.PublicError;
	
	    static createFrom(source: any = {}) {
	        return new CutJobStatusResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.job = this.convertValues(source["job"], jobs.Snapshot);
	        this.error = this.convertValues(source["error"], apperrors.PublicError);
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
	export class DetectionJobResponse {
	    job?: jobs.DetectionSnapshot;
	    error?: apperrors.PublicError;
	
	    static createFrom(source: any = {}) {
	        return new DetectionJobResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.job = this.convertValues(source["job"], jobs.DetectionSnapshot);
	        this.error = this.convertValues(source["error"], apperrors.PublicError);
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
	export class FileFailureDTO {
	    path: string;
	    reason: string;
	
	    static createFrom(source: any = {}) {
	        return new FileFailureDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.reason = source["reason"];
	    }
	}
	export class ListPeopleRequest {
	    workspaceId: number;
	    minDurationSec: number;
	    minConfidence: number;
	    page: number;
	    pageSize: number;
	
	    static createFrom(source: any = {}) {
	        return new ListPeopleRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspaceId = source["workspaceId"];
	        this.minDurationSec = source["minDurationSec"];
	        this.minConfidence = source["minConfidence"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	    }
	}
	export class PersonTrackDTO {
	    id: number;
	    videoId: number;
	    videoName: string;
	    startSec: number;
	    endSec: number;
	
	    static createFrom(source: any = {}) {
	        return new PersonTrackDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.videoId = source["videoId"];
	        this.videoName = source["videoName"];
	        this.startSec = source["startSec"];
	        this.endSec = source["endSec"];
	    }
	}
	export class PersonGroupDTO {
	    id: number;
	    label?: string;
	    thumbnailDataUrl: string;
	    startSec?: number;
	    trackCount: number;
	    videoCount: number;
	    totalDurationSec: number;
	    avgConfidence: number;
	    tracks: PersonTrackDTO[];
	
	    static createFrom(source: any = {}) {
	        return new PersonGroupDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.thumbnailDataUrl = source["thumbnailDataUrl"];
	        this.startSec = source["startSec"];
	        this.trackCount = source["trackCount"];
	        this.videoCount = source["videoCount"];
	        this.totalDurationSec = source["totalDurationSec"];
	        this.avgConfidence = source["avgConfidence"];
	        this.tracks = this.convertValues(source["tracks"], PersonTrackDTO);
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
	export class ListPeopleResponse {
	    items?: PersonGroupDTO[];
	    total: number;
	    error?: apperrors.PublicError;
	
	    static createFrom(source: any = {}) {
	        return new ListPeopleResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], PersonGroupDTO);
	        this.total = source["total"];
	        this.error = this.convertValues(source["error"], apperrors.PublicError);
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
	export class WorkspaceDTO {
	    id: number;
	    directory: string;
	    name: string;
	    createdAt: string;
	    updatedAt: string;
	    videoCount: number;
	    nullStartedAtCount: number;
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.directory = source["directory"];
	        this.name = source["name"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.videoCount = source["videoCount"];
	        this.nullStartedAtCount = source["nullStartedAtCount"];
	    }
	}
	export class ListWorkspacesResponse {
	    workspaces?: WorkspaceDTO[];
	    error?: apperrors.PublicError;
	
	    static createFrom(source: any = {}) {
	        return new ListWorkspacesResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspaces = this.convertValues(source["workspaces"], WorkspaceDTO);
	        this.error = this.convertValues(source["error"], apperrors.PublicError);
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
	export class WorkspaceLoadSummaryDTO {
	    totalScanned: number;
	    indexed: number;
	    nullStartedAtCount: number;
	    failures: FileFailureDTO[];
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceLoadSummaryDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalScanned = source["totalScanned"];
	        this.indexed = source["indexed"];
	        this.nullStartedAtCount = source["nullStartedAtCount"];
	        this.failures = this.convertValues(source["failures"], FileFailureDTO);
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
	export class LoadWorkspaceResponse {
	    workspace?: WorkspaceDTO;
	    summary?: WorkspaceLoadSummaryDTO;
	    error?: apperrors.PublicError;
	
	    static createFrom(source: any = {}) {
	        return new LoadWorkspaceResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspace = this.convertValues(source["workspace"], WorkspaceDTO);
	        this.summary = this.convertValues(source["summary"], WorkspaceLoadSummaryDTO);
	        this.error = this.convertValues(source["error"], apperrors.PublicError);
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
	export class VideoDTO {
	    id: number;
	    workspaceId: number;
	    name: string;
	    path: string;
	    startedAt?: string;
	    finishedAt?: string;
	    durationSeconds: number;
	
	    static createFrom(source: any = {}) {
	        return new VideoDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.workspaceId = source["workspaceId"];
	        this.name = source["name"];
	        this.path = source["path"];
	        this.startedAt = source["startedAt"];
	        this.finishedAt = source["finishedAt"];
	        this.durationSeconds = source["durationSeconds"];
	    }
	}
	export class PagedVideosResponse {
	    videos?: VideoDTO[];
	    page: number;
	    pageSize: number;
	    total: number;
	    error?: apperrors.PublicError;
	
	    static createFrom(source: any = {}) {
	        return new PagedVideosResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.videos = this.convertValues(source["videos"], VideoDTO);
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.total = source["total"];
	        this.error = this.convertValues(source["error"], apperrors.PublicError);
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
	
	export class PersonGroupResponse {
	    group?: PersonGroupDTO;
	    error?: apperrors.PublicError;
	
	    static createFrom(source: any = {}) {
	        return new PersonGroupResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.group = this.convertValues(source["group"], PersonGroupDTO);
	        this.error = this.convertValues(source["error"], apperrors.PublicError);
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
	
	export class PreviewCutsFromCSVRequest {
	    workspaceId: number;
	
	    static createFrom(source: any = {}) {
	        return new PreviewCutsFromCSVRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspaceId = source["workspaceId"];
	    }
	}
	export class PreviewCutsFromCSVResponse {
	    fileName?: string;
	    cuts?: CSVImportCutDTO[];
	    issues?: CSVImportIssueDTO[];
	    cancelled: boolean;
	    error?: apperrors.PublicError;
	
	    static createFrom(source: any = {}) {
	        return new PreviewCutsFromCSVResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fileName = source["fileName"];
	        this.cuts = this.convertValues(source["cuts"], CSVImportCutDTO);
	        this.issues = this.convertValues(source["issues"], CSVImportIssueDTO);
	        this.cancelled = source["cancelled"];
	        this.error = this.convertValues(source["error"], apperrors.PublicError);
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
	export class ShiftVideosTimeRequest {
	    workspaceId: number;
	    videoIds: number[];
	    shiftMilliseconds: number;
	
	    static createFrom(source: any = {}) {
	        return new ShiftVideosTimeRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspaceId = source["workspaceId"];
	        this.videoIds = source["videoIds"];
	        this.shiftMilliseconds = source["shiftMilliseconds"];
	    }
	}
	export class ShiftVideosTimeResponse {
	    updated: number;
	    error?: apperrors.PublicError;
	
	    static createFrom(source: any = {}) {
	        return new ShiftVideosTimeResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.updated = source["updated"];
	        this.error = this.convertValues(source["error"], apperrors.PublicError);
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
	export class StartCutRequest {
	    workspaceId: number;
	    name: string;
	    datetimeLocal: string;
	    secondsBefore: number;
	    secondsAfter: number;
	
	    static createFrom(source: any = {}) {
	        return new StartCutRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspaceId = source["workspaceId"];
	        this.name = source["name"];
	        this.datetimeLocal = source["datetimeLocal"];
	        this.secondsBefore = source["secondsBefore"];
	        this.secondsAfter = source["secondsAfter"];
	    }
	}
	export class StartCutResponse {
	    jobId?: string;
	    error?: apperrors.PublicError;
	
	    static createFrom(source: any = {}) {
	        return new StartCutResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.jobId = source["jobId"];
	        this.error = this.convertValues(source["error"], apperrors.PublicError);
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
	export class StartCutsFromCSVRequest {
	    workspaceId: number;
	    cuts: CSVImportCutDTO[];
	
	    static createFrom(source: any = {}) {
	        return new StartCutsFromCSVRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspaceId = source["workspaceId"];
	        this.cuts = this.convertValues(source["cuts"], CSVImportCutDTO);
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
	export class StartCutsFromCSVResponse {
	    jobId?: string;
	    error?: apperrors.PublicError;
	
	    static createFrom(source: any = {}) {
	        return new StartCutsFromCSVResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.jobId = source["jobId"];
	        this.error = this.convertValues(source["error"], apperrors.PublicError);
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
	export class StartDetectionRequest {
	    workspaceId: number;
	    sampleFps: number;
	
	    static createFrom(source: any = {}) {
	        return new StartDetectionRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspaceId = source["workspaceId"];
	        this.sampleFps = source["sampleFps"];
	    }
	}
	export class StartDetectionResponse {
	    jobId?: string;
	    error?: apperrors.PublicError;
	
	    static createFrom(source: any = {}) {
	        return new StartDetectionResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.jobId = source["jobId"];
	        this.error = this.convertValues(source["error"], apperrors.PublicError);
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
	export class StartPeopleCutRequest {
	    workspaceId: number;
	    groupIds: number[];
	    name: string;
	    secondsBefore: number;
	    secondsAfter: number;
	
	    static createFrom(source: any = {}) {
	        return new StartPeopleCutRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspaceId = source["workspaceId"];
	        this.groupIds = source["groupIds"];
	        this.name = source["name"];
	        this.secondsBefore = source["secondsBefore"];
	        this.secondsAfter = source["secondsAfter"];
	    }
	}
	
	export class VideoExplorerDTO {
	    id: number;
	    workspaceId: number;
	    name: string;
	    path: string;
	    relativePath: string;
	    relativeDir: string;
	    startedAt?: string;
	    finishedAt?: string;
	    durationSeconds: number;
	
	    static createFrom(source: any = {}) {
	        return new VideoExplorerDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.workspaceId = source["workspaceId"];
	        this.name = source["name"];
	        this.path = source["path"];
	        this.relativePath = source["relativePath"];
	        this.relativeDir = source["relativeDir"];
	        this.startedAt = source["startedAt"];
	        this.finishedAt = source["finishedAt"];
	        this.durationSeconds = source["durationSeconds"];
	    }
	}
	export class VideoPreviewResponse {
	    videoId: number;
	    dataUrl?: string;
	    cached: boolean;
	    error?: apperrors.PublicError;
	
	    static createFrom(source: any = {}) {
	        return new VideoPreviewResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.videoId = source["videoId"];
	        this.dataUrl = source["dataUrl"];
	        this.cached = source["cached"];
	        this.error = this.convertValues(source["error"], apperrors.PublicError);
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
	export class VideoTrackBoxesDTO {
	    trackId: number;
	    groupId: number;
	    label?: string;
	    startSec: number;
	    endSec: number;
	    boxes: number[][];
	
	    static createFrom(source: any = {}) {
	        return new VideoTrackBoxesDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.trackId = source["trackId"];
	        this.groupId = source["groupId"];
	        this.label = source["label"];
	        this.startSec = source["startSec"];
	        this.endSec = source["endSec"];
	        this.boxes = source["boxes"];
	    }
	}
	export class VideoTrackBoxesResponse {
	    videoId: number;
	    sampleFps: number;
	    tracks: VideoTrackBoxesDTO[];
	    error?: apperrors.PublicError;
	
	    static createFrom(source: any = {}) {
	        return new VideoTrackBoxesResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.videoId = source["videoId"];
	        this.sampleFps = source["sampleFps"];
	        this.tracks = this.convertValues(source["tracks"], VideoTrackBoxesDTO);
	        this.error = this.convertValues(source["error"], apperrors.PublicError);
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
	
	export class WorkspaceExplorerResponse {
	    workspaceId: number;
	    workspaceDirectory: string;
	    videos?: VideoExplorerDTO[];
	    error?: apperrors.PublicError;
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceExplorerResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspaceId = source["workspaceId"];
	        this.workspaceDirectory = source["workspaceDirectory"];
	        this.videos = this.convertValues(source["videos"], VideoExplorerDTO);
	        this.error = this.convertValues(source["error"], apperrors.PublicError);
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

