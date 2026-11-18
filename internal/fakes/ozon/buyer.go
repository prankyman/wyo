package ozon

// The made-up buyer every answer that lists postings carries. Ozon's lists return the buyer's name, phone, e-mail, address
// and comment with every FBS posting whatever the request asks for (OZC section 6), and the wyo server must drop them at the
// adapter's edge. The values are made up and recognisable, so that a test elsewhere can search the wyo server's journal, logs,
// database and answers for them with BuyerValues and find nothing.
const (
	BuyerName    = "Покупатель Тестовый"
	BuyerPhone   = "+70000000000"
	BuyerEmail   = "buyer@example.invalid"
	BuyerAddress = "ул. Выдуманная, д. 1"
	BuyerCity    = "Город"
	BuyerZipCode = "000000"
	BuyerComment = "Позвонить за час"
)

// BuyerValues lists every made-up value of the buyer.
func BuyerValues() []string {
	return []string{BuyerName, BuyerPhone, BuyerEmail, BuyerAddress, BuyerCity, BuyerZipCode, BuyerComment}
}

type wireCustomer struct {
	Name          string      `json:"name"`
	Phone         string      `json:"phone"`
	CustomerEmail string      `json:"customer_email,omitempty"`
	Address       wireAddress `json:"address"`
}

type wireAddress struct {
	AddressTail string `json:"address_tail"`
	City        string `json:"city"`
	ZipCode     string `json:"zip_code"`
	Comment     string `json:"comment"`
}

type wireAddressee struct {
	Name string `json:"name"`
}

// buyerOf is the made-up buyer as a posting carries it. The customer of the single posting (/v3/posting/fbs/get) has no
// e-mail in Ozon's file, and the customer of the list (/v4/posting/fbs/unfulfilled/list) has, so withEmail says which.
func buyerOf(withEmail bool) (wireCustomer, wireAddressee) {
	customer := wireCustomer{
		Name: BuyerName, Phone: BuyerPhone,
		Address: wireAddress{AddressTail: BuyerAddress, City: BuyerCity, ZipCode: BuyerZipCode, Comment: BuyerComment},
	}

	if withEmail {
		customer.CustomerEmail = BuyerEmail
	}

	return customer, wireAddressee{Name: BuyerName}
}
