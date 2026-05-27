export namespace claude {
	
	export class BarData {
	    accountName: string;
	    subscriptionType: string;
	    periodMessages: number;
	    periodPercent: number;
	    periodMsgLimit: number;
	    lastDataLabel: string;
	    lastDataMsgs: number;
	    hourlyPercent: number;
	    hourlyResetIn: string;
	    resetIn: string;
	    primaryModel: string;
	    status: string;
	    limitExceeded: boolean;
	    lastUpdated: number;
	
	    static createFrom(source: any = {}) {
	        return new BarData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.accountName = source["accountName"];
	        this.subscriptionType = source["subscriptionType"];
	        this.periodMessages = source["periodMessages"];
	        this.periodPercent = source["periodPercent"];
	        this.periodMsgLimit = source["periodMsgLimit"];
	        this.lastDataLabel = source["lastDataLabel"];
	        this.lastDataMsgs = source["lastDataMsgs"];
	        this.hourlyPercent = source["hourlyPercent"];
	        this.hourlyResetIn = source["hourlyResetIn"];
	        this.resetIn = source["resetIn"];
	        this.primaryModel = source["primaryModel"];
	        this.status = source["status"];
	        this.limitExceeded = source["limitExceeded"];
	        this.lastUpdated = source["lastUpdated"];
	    }
	}

}

export namespace config {
	
	export class AccountConfig {
	    name: string;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new AccountConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	    }
	}
	export class HotkeyConfig {
	    cycleMonitor: string;
	    toggleClickThrough: string;
	
	    static createFrom(source: any = {}) {
	        return new HotkeyConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cycleMonitor = source["cycleMonitor"];
	        this.toggleClickThrough = source["toggleClickThrough"];
	    }
	}
	export class Config {
	    monitor: number;
	    theme: string;
	    opacity: number;
	    refreshSeconds: number;
	    weeklyMsgLimit: number;
	    billingResetDay: number;
	    barHeight: number;
	    activeAccount: number;
	    accounts: AccountConfig[];
	    hotkeys: HotkeyConfig;
	    startWithWindows: boolean;
	    clickThrough: boolean;
	    appBarMode: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.monitor = source["monitor"];
	        this.theme = source["theme"];
	        this.opacity = source["opacity"];
	        this.refreshSeconds = source["refreshSeconds"];
	        this.weeklyMsgLimit = source["weeklyMsgLimit"];
	        this.billingResetDay = source["billingResetDay"];
	        this.barHeight = source["barHeight"];
	        this.activeAccount = source["activeAccount"];
	        this.accounts = this.convertValues(source["accounts"], AccountConfig);
	        this.hotkeys = this.convertValues(source["hotkeys"], HotkeyConfig);
	        this.startWithWindows = source["startWithWindows"];
	        this.clickThrough = source["clickThrough"];
	        this.appBarMode = source["appBarMode"];
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

export namespace syswin {
	
	export class MonitorInfo {
	    index: number;
	    left: number;
	    top: number;
	    width: number;
	    height: number;
	    physWidth: number;
	    dpiScale: number;
	    isPrimary: boolean;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new MonitorInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.left = source["left"];
	        this.top = source["top"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.physWidth = source["physWidth"];
	        this.dpiScale = source["dpiScale"];
	        this.isPrimary = source["isPrimary"];
	        this.name = source["name"];
	    }
	}

}

