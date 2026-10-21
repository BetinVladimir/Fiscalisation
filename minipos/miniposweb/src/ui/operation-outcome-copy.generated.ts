// Actual translategemma:12b catalogues; canonical sourceoverlay separate from sync-work regeneration.
export const operationOutcomeCatalogues:Record<string,Record<string,string>>={
  "en": {
    "confirmedTitle": "Operation completed",
    "confirmedBody": "The operation result is confirmed.",
    "failedTitle": "Operation not completed",
    "failedBody": "The operation did not complete. Check the details before continuing.",
    "pendingTitle": "Awaiting confirmation",
    "pendingBody": "The result is not confirmed. Do not repeat the payment; check the operation status.",
    "closeShiftBody": "Closing the shift requests a Z report. You can start a new shift on this register only after closure is confirmed.",
    "reconcileShiftBody": "Check the Z report result to confirm whether this shift is closed. A new shift on this register remains unavailable until closure is confirmed."
  },
  "bg": {
    "confirmedTitle": "Операцията е завършена",
    "confirmedBody": "Резултатът от операцията е потвърден.",
    "failedTitle": "Операцията не е завършена",
    "failedBody": "Операцията не беше завършена. Проверете детайлите, преди да продължите.",
    "pendingTitle": "Очаква се потвърждение",
    "pendingBody": "Резултатът все още не е потвърден. Не повтаряйте плащането; проверете статуса на операцията.",
    "closeShiftBody": "Затварянето на смяната изисква Z отчет. Нова смяна на този регистър може да бъде стартирана само след потвърждаване на затварянето.",
    "reconcileShiftBody": "Проверете резултата от Z отчета, за да потвърдите дали тази смяна е затворена. Нова смяна на този регистър остава недостъпна, докато затварянето не бъде потвърдено."
  },
  "hr": {
    "confirmedTitle": "Operacija je uspješno završena",
    "confirmedBody": "Rezultat operacije je potvrđen.",
    "failedTitle": "Operacija nije uspješno završena",
    "failedBody": "Operacija nije završena. Provjerite detalje prije nastavka.",
    "pendingTitle": "Čeka se potvrda",
    "pendingBody": "Rezultat nije potvrđen. Ne ponavljajte plaćanje; provjerite status operacije.",
    "closeShiftBody": "Zatvaranje smjene zahtijeva izvještaj Z. Novu smjenu na ovom uređaju možete započeti tek nakon potvrde zatvaranja.",
    "reconcileShiftBody": "Provjerite rezultat izvješća Z kako biste potvrdili je li ova smjena zatvorena. Nova smjena na ovom uređaju ostaje nedostupna dok se zatvaranje ne potvrdi."
  },
  "cs": {
    "confirmedTitle": "Operace dokončena",
    "confirmedBody": "Výsledek operace je potvrzen.",
    "failedTitle": "Operace nebyla dokončena",
    "failedBody": "Operace nebyla dokončena. Před pokračováním zkontrolujte podrobnosti.",
    "pendingTitle": "Čekání na potvrzení",
    "pendingBody": "Výsledek není potvrzen. Neopakujte platbu; zkontrolujte stav operace.",
    "closeShiftBody": "Ukončení směny vyžaduje Z report. Novou směnu na tomto zařízení můžete zahájit až po potvrzení ukončení.",
    "reconcileShiftBody": "Zkontrolujte výsledek Z reportu, abyste potvrdili, zda je tato směna ukončena. Nová směna na tomto zařízení zůstává nedostupná, dokud není ukončení potvrzeno."
  },
  "da": {
    "confirmedTitle": "Operation gennemført",
    "confirmedBody": "Resultatet af operationen er bekræftet.",
    "failedTitle": "Operation ikke gennemført",
    "failedBody": "Operationen blev ikke gennemført. Kontroller detaljerne, før du fortsætter.",
    "pendingTitle": "Afventer bekræftelse",
    "pendingBody": "Resultatet er ikke bekræftet. Gentag ikke betalingen; tjek status for operationen.",
    "closeShiftBody": "Lukning af skiftet anmoder om en Z-rapport. Du kan kun starte et nyt skift på denne registrering, efter at lukningen er bekræftet.",
    "reconcileShiftBody": "Kontroller resultatet af Z-rapporten for at bekræfte, om dette skift er lukket. Et nyt skift på denne registrering er stadig ikke tilgængeligt, før lukningen er bekræftet."
  },
  "nl": {
    "confirmedTitle": "Operatie voltooid",
    "confirmedBody": "Het resultaat van de operatie is bevestigd.",
    "failedTitle": "Operatie niet voltooid",
    "failedBody": "De operatie is niet voltooid. Controleer de details voordat u verdergaat.",
    "pendingTitle": "Wacht op bevestiging",
    "pendingBody": "Het resultaat is nog niet bevestigd. Herhaal de betaling niet; controleer de status van de operatie.",
    "closeShiftBody": "Het sluiten van de dienst vraagt om een Z-rapport. U kunt een nieuwe dienst op deze kassa pas starten nadat het sluiten is bevestigd.",
    "reconcileShiftBody": "Controleer het resultaat van het Z-rapport om te bevestigen of deze dienst is gesloten. Een nieuwe dienst op deze kassa blijft onbeschikbaar totdat het sluiten is bevestigd."
  },
  "et": {
    "confirmedTitle": "Operācija veiksmīgi pabeigta",
    "confirmedBody": "Operācijas rezultāts ir apstiprināts.",
    "failedTitle": "Operācija nav pabeigta",
    "failedBody": "Operācija nav pabeigta. Pārbaudiet detaļas, pirms turpināt.",
    "pendingTitle": "Gaida apstiprinājumu",
    "pendingBody": "Rezultāts nav apstiprināts. Neatkārtojiet maksājumu; pārbaudiet operācijas statusu.",
    "closeShiftBody": "Darba maiņas slēgšana ietver Z-atskaites ģenerēšanu. Jaunu darba maiņu šajā reģistrā var sākt tikai pēc tam, kad slēgšana ir apstiprināta.",
    "reconcileShiftBody": "Pārbaudiet Z-atskaites rezultātu, lai apstiprinātu, vai šī darba maiņa ir slēgta. Jauna darba maiņa šajā reģistrā nav pieejama, līdz slēgšana ir apstiprināta."
  },
  "fi": {
    "confirmedTitle": "Toiminto suoritettu",
    "confirmedBody": "Toiminnon tulos on vahvistettu.",
    "failedTitle": "Toiminto ei valmistunut",
    "failedBody": "Toiminto ei valmistunut. Tarkista tiedot ennen jatkamista.",
    "pendingTitle": "Odotetaan vahvistusta",
    "pendingBody": "Tulos ei ole vahvistettu. Älä toista maksua; tarkista toiminnon tila.",
    "closeShiftBody": "Vuoron sulkeminen vaatii Z-raportin. Uuden vuoron voi aloittaa tällä kassalla vasta, kun sulkeminen on vahvistettu.",
    "reconcileShiftBody": "Tarkista Z-raportin tulos varmistaaksesi, onko tämä vuoro suljettu. Uusi vuoro tällä kassalla ei ole käytettävissä ennen kuin sulkeminen on vahvistettu."
  },
  "fr": {
    "confirmedTitle": "Opération terminée",
    "confirmedBody": "Le résultat de l'opération est confirmé.",
    "failedTitle": "Opération non terminée",
    "failedBody": "L'opération n'a pas été effectuée. Vérifiez les détails avant de continuer.",
    "pendingTitle": "En attente de confirmation",
    "pendingBody": "Le résultat n'est pas confirmé. Ne répétez pas le paiement ; vérifiez le statut de l'opération.",
    "closeShiftBody": "La fermeture de la caisse demande un rapport Z. Vous ne pouvez démarrer une nouvelle caisse sur ce terminal qu'après confirmation de la fermeture.",
    "reconcileShiftBody": "Vérifiez le résultat du rapport Z pour confirmer si cette caisse est fermée. Une nouvelle caisse sur ce terminal reste indisponible jusqu'à confirmation de la fermeture."
  },
  "de": {
    "confirmedTitle": "Vorgang abgeschlossen",
    "confirmedBody": "Das Ergebnis des Vorgangs wurde bestätigt.",
    "failedTitle": "Vorgang nicht abgeschlossen",
    "failedBody": "Der Vorgang wurde nicht abgeschlossen. Überprüfen Sie die Details, bevor Sie fortfahren.",
    "pendingTitle": "Wartet auf Bestätigung",
    "pendingBody": "Das Ergebnis wurde noch nicht bestätigt. Wiederholen Sie die Zahlung nicht; überprüfen Sie den Status des Vorgangs.",
    "closeShiftBody": "Das Schließen der Schicht erfordert einen Z-Bericht. Eine neue Schicht an diesem Gerät kann erst nach Bestätigung der Schließung gestartet werden.",
    "reconcileShiftBody": "Überprüfen Sie das Ergebnis des Z-Berichts, um zu bestätigen, ob diese Schicht geschlossen wurde. Eine neue Schicht an diesem Gerät bleibt bis zur Bestätigung der Schließung nicht verfügbar."
  },
  "el": {
    "confirmedTitle": "Η λειτουργία ολοκληρώθηκε",
    "confirmedBody": "Το αποτέλεσμα της λειτουργίας έχει επιβεβαιωθεί.",
    "failedTitle": "Η λειτουργία δεν ολοκληρώθηκε",
    "failedBody": "Η λειτουργία δεν ολοκληρώθηκε. Ελέγξτε τις λεπτομέρειες πριν συνεχίσετε.",
    "pendingTitle": "Αναμονή επιβεβαίωσης",
    "pendingBody": "Το αποτέλεσμα δεν έχει επιβεβαιωθεί. Μην επαναλάβετε την πληρωμή. Ελέγξτε την κατάσταση της λειτουργίας.",
    "closeShiftBody": "Το κλείσιμο της βάρδιας απαιτεί μια αναφορά Z. Μπορείτε να ξεκινήσετε μια νέα βάρδια σε αυτό το ταμείο μόνο αφού το κλείσιμο επιβεβαιωθεί.",
    "reconcileShiftBody": "Ελέγξτε το αποτέλεσμα της αναφοράς Z για να επιβεβαιώσετε εάν η βάρδια έχει κλείσει. Μια νέα βάρδια σε αυτό το ταμείο παραμένει διαθέσιμη μόνο αφού το κλείσιμο επιβεβαιωθεί."
  },
  "hu": {
    "confirmedTitle": "Művelet sikeres",
    "confirmedBody": "A művelet eredménye megerősítve.",
    "failedTitle": "Művelet nem sikerült",
    "failedBody": "A művelet nem fejeződött be. Ellenőrizze a részleteket, mielőtt folytatná.",
    "pendingTitle": "Megerősítés alatt",
    "pendingBody": "Az eredmény még nem megerősítve. Ne ismételje meg a fizetést; ellenőrizze a művelet állapotát.",
    "closeShiftBody": "A műszak lezárása egy Z-jelentést igényel. Csak a lezárás megerősítése után kezdhet új műszakot ezen a kasszán.",
    "reconcileShiftBody": "Ellenőrizze a Z-jelentés eredményét annak megerősítéséhez, hogy ez a műszak le van-e zárva. A műszak újbóli megnyitása ezen a kasszán csak a lezárás megerősítése után lehetséges."
  },
  "ga": {
    "confirmedTitle": "İşlem tamamlandı",
    "confirmedBody": "İşlem sonucu doğrulandı.",
    "failedTitle": "İşlem tamamlanmadı",
    "failedBody": "İşlem tamamlanmadı. Devam etmeden önce ayrıntıları kontrol edin.",
    "pendingTitle": "Onay bekleniyor",
    "pendingBody": "Sonuç henüz doğrulanmadı. Ödemeyi tekrarlamayın; işlemin durumunu kontrol edin.",
    "closeShiftBody": "Vardiya kapatma işlemi, bir Z raporu talep eder. Bu kayıt cihazında yeni bir vardiyaya yalnızca kapanış doğrulandıktan sonra başlayabilirsiniz.",
    "reconcileShiftBody": "Vardiyanın kapatılıp kapatılmadığını doğrulamak için Z raporu sonucunu kontrol edin. Kapanış doğrulandığına kadar bu kayıt cihazında yeni bir vardiyaya başlanamaz."
  },
  "it": {
    "confirmedTitle": "Operação concluída",
    "confirmedBody": "O resultado da operação foi confirmado.",
    "failedTitle": "Operação não concluída",
    "failedBody": "A operação não foi concluída. Verifique os detalhes antes de continuar.",
    "pendingTitle": "Aguardando confirmação",
    "pendingBody": "O resultado não foi confirmado. Não repita o pagamento; verifique o status da operação.",
    "closeShiftBody": "Fechar o turno solicita um relatório Z. Você só pode iniciar um novo turno neste registrador após a confirmação do fechamento.",
    "reconcileShiftBody": "Verifique o resultado do relatório Z para confirmar se este turno foi fechado. Um novo turno neste registrador permanece indisponível até que o fechamento seja confirmado."
  },
  "lv": {
    "confirmedTitle": "Operācija veiksmīgi pabeigta",
    "confirmedBody": "Operācijas rezultāts ir apstiprināts.",
    "failedTitle": "Operācija nav pabeigta",
    "failedBody": "Operācija nav pabeigta. Pirms turpināšanas pārbaudiet detalizēto informāciju.",
    "pendingTitle": "Gaida apstiprinājumu",
    "pendingBody": "Rezultāts nav apstiprināts. Neatkārtojiet maksājumu; pārbaudiet operācijas statusu.",
    "closeShiftBody": "Darba maiņas slēgšana ietver Z-atskaites ģenerēšanu. Jaunu darba maiņu šajā reģistrā var sākt tikai pēc tam, kad slēgšana ir apstiprināta.",
    "reconcileShiftBody": "Pārbaudiet Z-atskaites rezultātu, lai apstiprinātu, vai šī darba maiņa ir slēgta. Jauna darba maiņa šajā reģistrā nav pieejama, līdz slēgšana ir apstiprināta."
  },
  "lt": {
    "confirmedTitle": "Operacija sėkmingai atlikta",
    "confirmedBody": "Operacijos rezultatas patvirtintas.",
    "failedTitle": "Operacija nebaigta",
    "failedBody": "Operacija nebaigta. Patikrinkite detales prieš tęsiant.",
    "pendingTitle": "Laukiama patvirtinimo",
    "pendingBody": "Rezultatas dar nepatvirtintas. Nelyginkite mokėjimo; patikrinkite operacijos būseną.",
    "closeShiftBody": "Pamainos uždarymas reikalauja Z ataskaitos. Naują pamainą šiame kasos aparate galite pradėti tik po to, kai uždarymas bus patvirtintas.",
    "reconcileShiftBody": "Patikrinkite Z ataskaitos rezultatą, kad patvirtintumėte, ar ši pamaina yra uždaryta. Nauja pamaina šiame kasos aparate lieka neprieinama, kol uždarymas nebus patvirtintas."
  },
  "mt": {
    "confirmedTitle": "Operazione completata",
    "confirmedBody": "Il risultato dell'operazione è confermato.",
    "failedTitle": "Operazione non completata",
    "failedBody": "L'operazione non è stata completata. Controllare i dettagli prima di procedere.",
    "pendingTitle": "In attesa di conferma",
    "pendingBody": "Il risultato non è confermato. Non ripetere il pagamento; controllare lo stato dell'operazione.",
    "closeShiftBody": "La chiusura del turno richiede un rapporto Z. È possibile avviare un nuovo turno su questo registratore solo dopo che la chiusura è stata confermata.",
    "reconcileShiftBody": "Controllare il risultato del rapporto Z per confermare se questo turno è chiuso. Un nuovo turno su questo registratore rimane non disponibile fino a quando la chiusura non è confermata."
  },
  "pl": {
    "confirmedTitle": "Operacja zakończona",
    "confirmedBody": "Wynik operacji został potwierdzony.",
    "failedTitle": "Operacja nie została zakończona",
    "failedBody": "Operacja nie została zakończona. Sprawdź szczegóły przed kontynuowaniem.",
    "pendingTitle": "Oczekuje na potwierdzenie",
    "pendingBody": "Wynik nie został potwierdzony. Nie powtarzaj płatności; sprawdź status operacji.",
    "closeShiftBody": "Zakończenie zmiany wymaga wygenerowania raportu Z. Możesz rozpocząć nową zmianę na tym urządzeniu tylko po potwierdzeniu zamknięcia.",
    "reconcileShiftBody": "Sprawdź wynik raportu Z, aby potwierdzić, czy ta zmiana została zamknięta. Rozpoczęcie nowej zmiany na tym urządzeniu pozostaje niedostępne do czasu potwierdzenia zamknięcia."
  },
  "pt": {
    "confirmedTitle": "Operação concluída",
    "confirmedBody": "O resultado da operação foi confirmado.",
    "failedTitle": "Operação não concluída",
    "failedBody": "A operação não foi concluída. Verifique os detalhes antes de continuar.",
    "pendingTitle": "Aguardando confirmação",
    "pendingBody": "O resultado não foi confirmado. Não repita o pagamento; verifique o status da operação.",
    "closeShiftBody": "Fechar o turno solicita um relatório Z. Você só pode iniciar um novo turno neste registrador após a confirmação do fechamento.",
    "reconcileShiftBody": "Verifique o resultado do relatório Z para confirmar se este turno está fechado. Um novo turno neste registrador permanece indisponível até que o fechamento seja confirmado."
  },
  "ro": {
    "confirmedTitle": "Operațiune finalizată",
    "confirmedBody": "Rezultatul operațiunii este confirmat.",
    "failedTitle": "Operațiune nereușită",
    "failedBody": "Operațiunea nu a fost finalizată. Verificați detaliile înainte de a continua.",
    "pendingTitle": "În așteptarea confirmării",
    "pendingBody": "Rezultatul nu este confirmat. Nu repetați plata; verificați starea operațiunii.",
    "closeShiftBody": "Închiderea turei solicită un raport Z. Puteți începe o nouă tură pe acest registru numai după ce închiderea este confirmată.",
    "reconcileShiftBody": "Verificați rezultatul raportului Z pentru a confirma dacă această tură este închisă. O nouă tură pe acest registru rămâne indisponibilă până la confirmarea închiderii."
  },
  "sk": {
    "confirmedTitle": "Operácia dokončená",
    "confirmedBody": "Výsledok operácie je potvrdený.",
    "failedTitle": "Operácia nebola dokončená",
    "failedBody": "Operácia sa nedokončila. Skontrolujte detaily pred pokračovaním.",
    "pendingTitle": "Čaká sa na potvrdenie",
    "pendingBody": "Výsledok nie je potvrdený. Neopakovujte platbu; skontrolujte stav operácie.",
    "closeShiftBody": "Ukončenie zmeny vyžaduje generovanie Z reportu. Novú zmenu na tomto pokladni môže začať až po potvrdení ukončenia.",
    "reconcileShiftBody": "Skontrolujte výsledok Z reportu, aby ste potvrdili, či je táto zmena ukončená. Nová zmena na tejto pokladni zostáva nedostupná, kým nie je ukončenie potvrdené."
  },
  "sl": {
    "confirmedTitle": "Operacija je uspešno zaključena",
    "confirmedBody": "Rezultat operacije je potrjen.",
    "failedTitle": "Operacija ni bila uspešno zaključena",
    "failedBody": "Operacija ni bila uspešno zaključena. Preverite podrobnosti, preden nadaljujete.",
    "pendingTitle": "Čakanje na potrditev",
    "pendingBody": "Rezultat še ni potrjen. Ne ponavljajte plačila; preverite stanje operacije.",
    "closeShiftBody": "Zapiranje izmene zahteva poročilo Z. Na tem registratorju lahko začnete novo izmeno šele po potrditvi zaprtja.",
    "reconcileShiftBody": "Preverite rezultat poročila Z, da potrdite, ali je ta izmena zaprta. Na tem registratorju nova izmena ostane nedostopna, dokler zaprtje ni potrjeno."
  },
  "es": {
    "confirmedTitle": "Operación completada",
    "confirmedBody": "El resultado de la operación está confirmado.",
    "failedTitle": "Operación no completada",
    "failedBody": "La operación no se completó. Revise los detalles antes de continuar.",
    "pendingTitle": "En espera de confirmación",
    "pendingBody": "El resultado no está confirmado. No repita el pago; verifique el estado de la operación.",
    "closeShiftBody": "Cerrar la jornada requiere un informe Z. Puede iniciar una nueva jornada en este registrador solo después de que se confirme el cierre.",
    "reconcileShiftBody": "Revise el resultado del informe Z para confirmar si esta jornada está cerrada. Una nueva jornada en este registrador permanece inhabilitada hasta que se confirme el cierre."
  },
  "sv": {
    "confirmedTitle": "Operationen genomförd",
    "confirmedBody": "Resultatet av operationen är bekräftat.",
    "failedTitle": "Operationen genomfördes inte",
    "failedBody": "Operationen slutfördes inte. Kontrollera detaljerna innan du fortsätter.",
    "pendingTitle": "Väntar på bekräftelse",
    "pendingBody": "Resultatet är inte bekräftat. Upprepa inte betalningen; kontrollera operationens status.",
    "closeShiftBody": "Att stänga skiftet kräver en Z-rapport. Du kan inte starta ett nytt skift på denna enhet förrän stängningen är bekräftad.",
    "reconcileShiftBody": "Kontrollera resultatet av Z-rapporten för att bekräfta om detta skift är stängt. Ett nytt skift på denna enhet är inte tillgängligt förrän stängningen är bekräftad."
  },
  "ru": {
    "confirmedTitle": "Операция завершена",
    "confirmedBody": "Результат операции подтвержден.",
    "failedTitle": "Операция не завершена",
    "failedBody": "Операция не была завершена. Проверьте детали перед продолжением.",
    "pendingTitle": "Ожидание подтверждения",
    "pendingBody": "Результат не подтвержден. Не повторяйте платеж; проверьте статус операции.",
    "closeShiftBody": "Закрытие смены требует формирования Z-отчета. Начать новую смену на этом терминале можно только после подтверждения закрытия.",
    "reconcileShiftBody": "Проверьте результат Z-отчета, чтобы убедиться, что смена закрыта. Начало новой смены на этом терминале остается недоступным до подтверждения закрытия."
  }
};
