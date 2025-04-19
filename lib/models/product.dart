class Product {
  final int id;
  final String name;
  final String description;
  final double price;
  final int categoryId;
  final List<String> imageUrls;

  Product({
    required this.id,
    required this.name,
    required this.description,
    required this.price,
    required this.categoryId,
    required this.imageUrls,
  });

  factory Product.fromJson(Map<String, dynamic> json) => Product(
        id: json['id'],
        name: json['name'],
        description: json['description'],
        price: json['price'].toDouble(),
        categoryId: json['category_id'],
        imageUrls:
            List<String>.from(json['images'].map((img) => img['image_url'])),
      );
}

class ProductCreate {
  final String name;
  final String description;
  final double price;
  final int categoryId;

  ProductCreate({
    required this.name,
    required this.description,
    required this.price,
    required this.categoryId,
  });

  Map<String, dynamic> toJson() => {
        'name': name,
        'description': description,
        'price': price,
        'category_id': categoryId,
      };
}
