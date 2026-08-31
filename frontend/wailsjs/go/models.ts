export namespace main {
	
	export class DRELinha {
	    categoria: string;
	    tipo: string;
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new DRELinha(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.categoria = source["categoria"];
	        this.tipo = source["tipo"];
	        this.total = source["total"];
	    }
	}
	export class DRE {
	    linhas: DRELinha[];
	    receitaBruta: number;
	    despesas: number;
	    resultado: number;
	
	    static createFrom(source: any = {}) {
	        return new DRE(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.linhas = this.convertValues(source["linhas"], DRELinha);
	        this.receitaBruta = source["receitaBruta"];
	        this.despesas = source["despesas"];
	        this.resultado = source["resultado"];
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
	
	export class FluxoCaixaLinha {
	    mes: string;
	    entradas: number;
	    saidas: number;
	    saldoPeriodo: number;
	    saldoAcumulado: number;
	
	    static createFrom(source: any = {}) {
	        return new FluxoCaixaLinha(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mes = source["mes"];
	        this.entradas = source["entradas"];
	        this.saidas = source["saidas"];
	        this.saldoPeriodo = source["saldoPeriodo"];
	        this.saldoAcumulado = source["saldoAcumulado"];
	    }
	}
	export class FluxoMes {
	    mes: string;
	    entradas: number;
	    saidas: number;
	
	    static createFrom(source: any = {}) {
	        return new FluxoMes(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mes = source["mes"];
	        this.entradas = source["entradas"];
	        this.saidas = source["saidas"];
	    }
	}
	export class Resumo {
	    saldoAtual: number;
	    totalAPagar: number;
	    totalAReceber: number;
	    proximosVencimentos: model.Lancamento[];
	    fluxoMensal: FluxoMes[];
	    hoje: string;
	
	    static createFrom(source: any = {}) {
	        return new Resumo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.saldoAtual = source["saldoAtual"];
	        this.totalAPagar = source["totalAPagar"];
	        this.totalAReceber = source["totalAReceber"];
	        this.proximosVencimentos = this.convertValues(source["proximosVencimentos"], model.Lancamento);
	        this.fluxoMensal = this.convertValues(source["fluxoMensal"], FluxoMes);
	        this.hoje = source["hoje"];
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

export namespace model {
	
	export class Categoria {
	    id: number;
	    nome: string;
	    tipo: string;
	
	    static createFrom(source: any = {}) {
	        return new Categoria(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.nome = source["nome"];
	        this.tipo = source["tipo"];
	    }
	}
	export class Conta {
	    id: number;
	    nome: string;
	    saldoInicial: number;
	
	    static createFrom(source: any = {}) {
	        return new Conta(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.nome = source["nome"];
	        this.saldoInicial = source["saldoInicial"];
	    }
	}
	export class Lancamento {
	    id: number;
	    tipo: string;
	    descricao: string;
	    categoriaId?: number;
	    contaId?: number;
	    valor: number;
	    dataVencimento: string;
	    dataPagamento: string;
	    observacoes: string;
	    status?: string;
	
	    static createFrom(source: any = {}) {
	        return new Lancamento(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.tipo = source["tipo"];
	        this.descricao = source["descricao"];
	        this.categoriaId = source["categoriaId"];
	        this.contaId = source["contaId"];
	        this.valor = source["valor"];
	        this.dataVencimento = source["dataVencimento"];
	        this.dataPagamento = source["dataPagamento"];
	        this.observacoes = source["observacoes"];
	        this.status = source["status"];
	    }
	}
	export class LancamentoFiltro {
	    tipo: string;
	    status: string;
	    dataInicio: string;
	    dataFim: string;
	
	    static createFrom(source: any = {}) {
	        return new LancamentoFiltro(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tipo = source["tipo"];
	        this.status = source["status"];
	        this.dataInicio = source["dataInicio"];
	        this.dataFim = source["dataFim"];
	    }
	}
	export class LancamentoInput {
	    tipo: string;
	    descricao: string;
	    categoriaId?: number;
	    contaId?: number;
	    valor: number;
	    dataVencimento: string;
	    observacoes: string;
	
	    static createFrom(source: any = {}) {
	        return new LancamentoInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tipo = source["tipo"];
	        this.descricao = source["descricao"];
	        this.categoriaId = source["categoriaId"];
	        this.contaId = source["contaId"];
	        this.valor = source["valor"];
	        this.dataVencimento = source["dataVencimento"];
	        this.observacoes = source["observacoes"];
	    }
	}

}

export namespace updater {
	
	export class Atualizacao {
	    versaoAtual: string;
	    versaoNova: string;
	    notas: string;
	    url: string;
	    // Go type: time
	    publicada: any;
	
	    static createFrom(source: any = {}) {
	        return new Atualizacao(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.versaoAtual = source["versaoAtual"];
	        this.versaoNova = source["versaoNova"];
	        this.notas = source["notas"];
	        this.url = source["url"];
	        this.publicada = this.convertValues(source["publicada"], null);
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

