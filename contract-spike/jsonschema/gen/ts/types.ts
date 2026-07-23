export interface Actor {
    actorId:    string;
    attributes: { [key: string]: number };
    moduleData: { [key: string]: unknown };
    moduleId:   string;
    name:       string;
    resources:  { [key: string]: ActorSchema };
}

export interface ActorSchema {
    current: number;
    max:     number;
}

export interface AttackRolled {
    attackerId: string;
    expression: string;
    modifiers:  ModifierElement[];
    outcome:    string;
    rolls:      RollElement[];
    targetId:   string;
    total:      number;
    versus:     string;
}

export interface ModifierElement {
    source: string;
    value:  number;
}

export interface RollElement {
    die:    number;
    result: number;
}

export interface MoveTokenRequest {
    to:      To;
    tokenId: string;
    [property: string]: unknown;
}

export interface To {
    x: number;
    y: number;
    [property: string]: unknown;
}

export interface TokenMoved {
    from:    From;
    sceneId: string;
    to:      From;
    tokenId: string;
}

export interface From {
    x: number;
    y: number;
}
