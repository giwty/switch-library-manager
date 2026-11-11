import test from 'node:test';
import assert from 'node:assert/strict';
import {convertNameCase, gameNameAction} from '../frontend/game_name_editor.js';

test('case tools preserve Unicode, spacing and punctuation', () => {
    assert.equal(convertNameCase('ÅÄÖ 日本語 Pokémon', 'upper'), 'ÅÄÖ 日本語 POKÉMON');
    assert.equal(convertNameCase('ÅÄÖ 日本語 Pokémon', 'lower'), 'åäö 日本語 pokémon');
    assert.equal(convertNameCase('  “hELLO”  wORLD!\tÅÄÖ', 'title'), '  “Hello”  World!\tÅäö');
    assert.equal(convertNameCase('  “hELLO” wORLD ÅÄÖ', 'sentence'), '  “Hello” world åäö');
});

test('each game table resolves its name action without editing DLC names', () => {
    const id = '0100ABCDEF123000';
    for (const [kind, data] of [['games', {titleId: id}], ['missing_games', {titleId: id}], ['missing_updates', {Attributes: {id}}], ['missing_dlc', {game: {id}, dlc: {id: '0100abcdef124001'}}]]) {
        assert.deepEqual(gameNameAction(data, kind), {type: 'game-name', titleID: id.toLowerCase(), label: 'Edit game name…'});
    }
    assert.equal(gameNameAction({titleId: 'bad'}, 'games'), null);
});
