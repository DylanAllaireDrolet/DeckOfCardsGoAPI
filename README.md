# TP1

Voici comment utiliser le programme :

Pour créer plusieurs decks, il faut utiliser l'URL suivante :

```http
http://localhost:8080/deck/new?count=3
```

Pour créer un deck, il faut utiliser l'URL suivante :

```http
http://localhost:8080/deck/new
```

Pour mélanger un deck, il faut utiliser l'URL suivante :

```http
http://localhost:8080/deck/shuffle/{deckId}
```

Pour tirer une carte d'un deck, il faut utiliser l'URL suivante :

```http
http://localhost:8080/deck/draw/{deckId}
```

Pour tirer plusieurs cartes d'un deck, il faut utiliser l'URL suivante :

```http
http://localhost:8080/deck/draw/{deckId}?nbrCarte=3
```

Pour ajouter une carte au deck, il faut utiliser l'URL suivante (1-13 et h, d, c, s):

```http
http://localhost:8080/deck/add/{deckId}?=1h
```

Pour ajouter plusieurs cartes au deck, il faut utiliser l'URL suivante :

```http
http://localhost:8080/deck/add/{deckId}?=1h,2h,3h
```

Pour afficher une carte, il faut utiliser l'URL suivante (1-13 et h, d, c, s) :

```http
http://localhost:8080/static/1h
```
