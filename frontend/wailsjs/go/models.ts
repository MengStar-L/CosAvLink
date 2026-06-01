export namespace model {
	
	export class Magnet {
	    name: string;
	    link: string;
	    size: string;
	    date: string;
	    tags: string[];
	    source?: string;
	
	    static createFrom(source: any = {}) {
	        return new Magnet(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.link = source["link"];
	        this.size = source["size"];
	        this.date = source["date"];
	        this.tags = source["tags"];
	        this.source = source["source"];
	    }
	}
	export class MagnetResult {
	    code: string;
	    query?: string;
	    magnets: Magnet[];
	    detailUrl: string;
	    blocked: boolean;
	    note: string;
	
	    static createFrom(source: any = {}) {
	        return new MagnetResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.query = source["query"];
	        this.magnets = this.convertValues(source["magnets"], Magnet);
	        this.detailUrl = source["detailUrl"];
	        this.blocked = source["blocked"];
	        this.note = source["note"];
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
	export class Video {
	    title: string;
	    code: string;
	    cover: string;
	    detailUrl: string;
	
	    static createFrom(source: any = {}) {
	        return new Video(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.code = source["code"];
	        this.cover = source["cover"];
	        this.detailUrl = source["detailUrl"];
	    }
	}

}

