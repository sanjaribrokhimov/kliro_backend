clickUrl  string
full document https://docs.click.uz/en/click-button, Example of generated URL https://my.click.uz/services/pay?service_id={service_id}&merchant_id={merchant_id}&amount={amount}&transaction_param={transaction_param}&return_url={return_url}

Чтобы перенаправить на страницу оплаты CLICK, вам необходимо создать кнопку (ссылка) на следующий адрес:
https://my.click.uz/services/pay?service_id={service_id}&merchant_id={merchant_id}&amount={amount}&transaction_param={transaction_param}&return_url={return_url}&card_type={card_type} 

# 	Имя параметра 	Тип данных 	Описание
1 	mersk_id 	Обязательный 	Торговый ID
2 	merchant_user_id 	необязательный 	ID пользователя для купеческой системы
3 	service_id 	Обязательный 	Торговый Сервис ID
4 	транзакция_param 	Обязательный 	ID заказа (для покупок в Интернете) / личный кабинет / вход в биллинг поставщика. Соответствует merchant_trans_id от SHOP-API
5 	сумма 	Обязательный 	Сумма сделки (формат: N.NN)
6 	return_url 	необязательный 	Ссылка, куда пользователь будет перенаправлен после оплаты
7 	card_тип 	необязательный 	Тип платежной системы (ZZcard, humo)

