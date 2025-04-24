import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../models/product.dart';
import '../models/review.dart';
import '../services/product_service.dart';
import '../services/order_service.dart';
import 'payment_screen.dart';

class ProductDetailScreen extends StatefulWidget {
  final int productId;

  const ProductDetailScreen({super.key, required this.productId});

  @override
  _ProductDetailScreenState createState() => _ProductDetailScreenState();
}

class _ProductDetailScreenState extends State<ProductDetailScreen> {
  late Future<Product> _productFuture;
  late Future<List<Review>> _reviewsFuture;
  int _newRating = 0;
  final TextEditingController _commentController = TextEditingController();

  @override
  void initState() {
    super.initState();
    _loadData();
  }

  void _loadData() {
    _productFuture = Provider.of<ProductService>(context, listen: false)
        .getProductById(widget.productId);
    _reviewsFuture = Provider.of<ProductService>(context, listen: false)
        .getReviewsForProduct(widget.productId);
  }

  double _calculateAverage(List<Review> reviews) {
    if (reviews.isEmpty) return 0;
    return reviews.map((r) => r.rating).reduce((a, b) => a + b) /
        reviews.length;
  }

  Widget _buildRatingStars(double avgRating) {
    int fullStars = avgRating.floor();
    bool halfStar = (avgRating - fullStars) >= 0.5;
    return Row(
      children: List.generate(5, (index) {
        if (index < fullStars) {
          return const Icon(Icons.star, size: 20, color: Colors.amber);
        } else if (index == fullStars && halfStar) {
          return const Icon(Icons.star_half, size: 20, color: Colors.amber);
        } else {
          return const Icon(Icons.star_border, size: 20, color: Colors.amber);
        }
      }),
    );
  }

  Future<void> _submitReview() async {
    if (_newRating == 0) return; // require rating
    try {
      await Provider.of<ProductService>(context, listen: false).addReview(
        productId: widget.productId,
        rating: _newRating,
        comment:
            _commentController.text.isNotEmpty ? _commentController.text : null,
      );
      _commentController.clear();
      setState(() {
        _newRating = 0;
        _reviewsFuture = Provider.of<ProductService>(context, listen: false)
            .getReviewsForProduct(widget.productId);
      });
      ScaffoldMessenger.of(context)
          .showSnackBar(const SnackBar(content: Text('Спасибо за отзыв!')));
    } catch (e) {
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text('Ошибка: $e')));
    }
  }

  Widget _buildReviewForm() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const Text('Оставить отзыв',
            style: TextStyle(fontSize: 18, fontWeight: FontWeight.w600)),
        const SizedBox(height: 8),
        Row(
          children: List.generate(5, (index) {
            int starIndex = index + 1;
            return IconButton(
              icon: Icon(
                _newRating >= starIndex ? Icons.star : Icons.star_border,
                color: Colors.amber,
              ),
              onPressed: () => setState(() => _newRating = starIndex),
            );
          }),
        ),
        TextField(
          controller: _commentController,
          decoration: const InputDecoration(
            hintText: 'Комментарий (необязательно)',
            border: OutlineInputBorder(),
          ),
          maxLines: 3,
        ),
        const SizedBox(height: 8),
        ElevatedButton(
          onPressed: _submitReview,
          child: const Text('Отправить'),
        ),
      ],
    );
  }

  Widget _buildReviewsSection(List<Review> reviews) {
    double avg = _calculateAverage(reviews);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const Text('Отзывы',
            style: TextStyle(fontSize: 18, fontWeight: FontWeight.w600)),
        const SizedBox(height: 4),
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            _buildRatingStars(avg),
            Text('(${reviews.length})'),
          ],
        ),
        const SizedBox(height: 12),
        if (reviews.isEmpty) const Text('Пока нет отзывов'),
        ...reviews.map((r) => Card(
              margin: const EdgeInsets.symmetric(vertical: 4),
              child: ListTile(
                title: _buildRatingStars(r.rating.toDouble()),
                subtitle: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    if (r.comment != null) Text(r.comment!),
                    Text(
                      r.createdAt.toLocal().toString().split(' ')[0],
                      style: const TextStyle(fontSize: 12, color: Colors.grey),
                    ),
                  ],
                ),
              ),
            ))
      ],
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: FutureBuilder<Product>(
        future: _productFuture,
        builder: (context, snapshot) {
          if (snapshot.connectionState == ConnectionState.waiting) {
            return const Center(child: CircularProgressIndicator());
          }
          if (snapshot.hasError) {
            return Center(child: Text('Ошибка: ${snapshot.error}'));
          }
          if (!snapshot.hasData) {
            return const Center(child: Text('Продукт не найден'));
          }
          final product = snapshot.data!;
          return Stack(
            children: [
              CustomScrollView(
                slivers: [
                  SliverAppBar(
                    expandedHeight: 320,
                    pinned: true,
                    elevation: 2,
                    backgroundColor: Colors.white,
                    flexibleSpace: FlexibleSpaceBar(
                      background: _buildImageGallery(product),
                    ),
                    leading: IconButton(
                      icon: const Icon(Icons.arrow_back, color: Colors.black),
                      onPressed: () => Navigator.pop(context),
                    ),
                  ),
                  SliverToBoxAdapter(
                    child: Padding(
                      padding: const EdgeInsets.all(20),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Container(
                            decoration: BoxDecoration(
                              color: Colors.white,
                              borderRadius: BorderRadius.circular(16),
                              boxShadow: const [
                                BoxShadow(
                                    color: Colors.black12,
                                    blurRadius: 12,
                                    offset: Offset(0, 4))
                              ],
                            ),
                            padding: const EdgeInsets.all(20),
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Text(product.name,
                                    style: const TextStyle(
                                        fontSize: 26,
                                        fontWeight: FontWeight.bold)),
                                const SizedBox(height: 8),
                                Text('${product.price.toStringAsFixed(2)} ₸',
                                    style: const TextStyle(
                                        fontSize: 22,
                                        fontWeight: FontWeight.w600,
                                        color: Colors.green)),
                                const Divider(height: 32),
                                const Text('Описание',
                                    style: TextStyle(
                                        fontSize: 18,
                                        fontWeight: FontWeight.w500)),
                                const SizedBox(height: 8),
                                Text(product.description,
                                    style: const TextStyle(
                                        fontSize: 16,
                                        height: 1.5,
                                        color: Colors.black87)),
                                const SizedBox(height: 16),
                                FutureBuilder<List<Review>>(
                                  future: _reviewsFuture,
                                  builder: (context, revSnap) {
                                    if (revSnap.connectionState ==
                                        ConnectionState.waiting) {
                                      return const Center(
                                          child: CircularProgressIndicator());
                                    }
                                    if (revSnap.hasError) {
                                      return Text(
                                          'Ошибка загрузки отзывов: ${revSnap.error}');
                                    }
                                    return _buildReviewsSection(revSnap.data!);
                                  },
                                ),
                                const Divider(height: 32),
                                _buildReviewForm(),
                              ],
                            ),
                          ),
                          const SizedBox(height: 100),
                        ],
                      ),
                    ),
                  ),
                ],
              ),
              _buildBottomActionBar(context, product),
            ],
          );
        },
      ),
    );
  }

  Widget _buildImageGallery(Product product) {
    if (product.imageUrls.isEmpty) {
      return const Center(child: Text('Нет изображений'));
    }
    return PageView.builder(
      itemCount: product.imageUrls.length,
      itemBuilder: (context, index) => ClipRRect(
        borderRadius: const BorderRadius.only(
            bottomLeft: Radius.circular(32), bottomRight: Radius.circular(32)),
        child: Image.network(
          'http://172.20.10.2:8000${product.imageUrls[index]}',
          fit: BoxFit.cover,
          loadingBuilder: (context, child, progress) {
            if (progress == null) return child;
            return const Center(child: CircularProgressIndicator());
          },
        ),
      ),
    );
  }

  Widget _buildBottomActionBar(BuildContext context, Product product) {
    return Positioned(
      bottom: 0,
      left: 0,
      right: 0,
      child: SafeArea(
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
          decoration: BoxDecoration(color: Colors.white, boxShadow: const [
            BoxShadow(
                color: Colors.black12, blurRadius: 10, offset: Offset(0, -2))
          ]),
          child: Row(
            children: [
              Expanded(
                child: ElevatedButton.icon(
                  icon: const Icon(Icons.shopping_cart),
                  label: const Text('В корзину'),
                  style: ElevatedButton.styleFrom(
                      backgroundColor: Colors.deepOrange,
                      foregroundColor: Colors.white,
                      padding: const EdgeInsets.symmetric(vertical: 16),
                      shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(12))),
                  onPressed: () async {
                    try {
                      await Provider.of<ProductService>(context, listen: false)
                          .addToCart(product.id);
                      ScaffoldMessenger.of(context).showSnackBar(
                          const SnackBar(content: Text('Добавлено в корзину')));
                      Navigator.pushNamed(context, '/my-cart');
                    } catch (e) {
                      ScaffoldMessenger.of(context).showSnackBar(
                          SnackBar(content: Text('Ошибка при добавлении: $e')));
                    }
                  },
                ),
              ),
              const SizedBox(width: 12),
              ElevatedButton(
                child: const Text('Купить', style: TextStyle(fontSize: 16)),
                style: ElevatedButton.styleFrom(
                    backgroundColor: Colors.green,
                    foregroundColor: Colors.white,
                    padding: const EdgeInsets.symmetric(
                        horizontal: 24, vertical: 16),
                    shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(12))),
                onPressed: () async {
                  try {
                    final orderService =
                        Provider.of<OrderService>(context, listen: false);
                    final response = await orderService.createOrder([
                      {'product_id': product.id, 'quantity': 1}
                    ], 'Адрес доставки');
                    final total = (response['total'] as num).toDouble();
                    Navigator.push(
                      context,
                      MaterialPageRoute(
                        builder: (_) => PaymentScreen(
                            orderId: response['id'] as int, orderTotal: total),
                      ),
                    );
                  } catch (e) {
                    ScaffoldMessenger.of(context).showSnackBar(
                        SnackBar(content: Text('Ошибка создания заказа: $e')));
                  }
                },
              ),
            ],
          ),
        ),
      ),
    );
  }
}
