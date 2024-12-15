import 'package:flutter/material.dart';
import 'package:google_fonts/google_fonts.dart';

class MyRentalsScreen extends StatefulWidget {
  @override
  _MyRentalsScreenState createState() => _MyRentalsScreenState();
}

class _MyRentalsScreenState extends State<MyRentalsScreen>
    with SingleTickerProviderStateMixin {
  late TabController _tabController;

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 2, vsync: this);
  }

  @override
  void dispose() {
    _tabController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: Text('Мои аренды',
            style: GoogleFonts.montserrat(
                fontSize: 22, fontWeight: FontWeight.w600)),
        backgroundColor: Color(0xFF6C9942),
        elevation: 0, // Убираем тень
        bottom: TabBar(
          controller: _tabController,
          indicatorColor: Colors.white,
          labelStyle: GoogleFonts.montserrat(fontWeight: FontWeight.w600),
          unselectedLabelColor:
              Colors.black38, // Цвет текста неактивных вкладок
          labelColor: Colors.white, // Цвет текста активной вкладки
          tabs: [
            Tab(
              text: 'Действующие',
            ),
            Tab(
              text: 'Предыдущие',
            ),
          ],
        ),
      ),
      body: TabBarView(
        controller: _tabController,
        children: [
          _buildActiveRentals(),
          _buildPreviousRentals(),
        ],
      ),
    );
  }

  Widget _buildActiveRentals() {
    return Padding(
      padding: const EdgeInsets.all(16.0),
      child: AnimatedSwitcher(
        duration: const Duration(milliseconds: 300),
        child: ListView(
          key: ValueKey<int>(0),
          children: [
            _rentalTile('Бокс #101', 'Срок аренды: до 30 ноября 2024', 'XS'),
            _rentalTile('Бокс #102', 'Срок аренды: до 15 декабря 2024', 'S'),
          ],
        ),
      ),
    );
  }

  Widget _buildPreviousRentals() {
    return Padding(
      padding: const EdgeInsets.all(16.0),
      child: AnimatedSwitcher(
        duration: const Duration(milliseconds: 300),
        child: ListView(
          key: ValueKey<int>(1),
          children: [
            _rentalTile('Бокс #99', 'Завершено: 15 октября 2024', 'XL'),
            _rentalTile('Бокс #98', 'Завершено: 1 сентября 2024', 'M'),
          ],
        ),
      ),
    );
  }

  // Модальное окно для отображения информации об аренде
  void _showRentalDetails(
      BuildContext context, String title, String subtitle, String size) {
    showDialog(
      context: context,
      builder: (BuildContext context) {
        return AlertDialog(
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(20),
          ),
          title: Text(
            title,
            style: GoogleFonts.montserrat(
                fontSize: 20, fontWeight: FontWeight.bold),
          ),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                subtitle,
                style:
                    GoogleFonts.openSans(fontSize: 16, color: Colors.black87),
              ),
              SizedBox(height: 16),
              Text(
                'Размер: $size',
                style:
                    GoogleFonts.openSans(fontSize: 16, color: Colors.black87),
              ),
            ],
          ),
          actions: [
            TextButton(
              onPressed: () {
                Navigator.of(context).pop(); // Закрыть диалог
              },
              child: Text(
                'Закрыть',
                style: GoogleFonts.montserrat(fontSize: 16),
              ),
            ),
          ],
        );
      },
    );
  }

  Widget _rentalTile(String title, String subtitle, String size) {
    return Card(
      margin: const EdgeInsets.symmetric(vertical: 10),
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(20),
      ),
      elevation: 5,
      shadowColor: Colors.black38,
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          borderRadius: BorderRadius.circular(20),
          onTap: () {
            // Открыть модальное окно с информацией
            _showRentalDetails(context, title, subtitle, size);
          },
          child: Container(
            padding: const EdgeInsets.symmetric(vertical: 15, horizontal: 20),
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(20),
              color:
                  Color(0xFF6C9942).withOpacity(0.1), // Легкий фон для карточки
            ),
            child: ListTile(
              title: Text(
                title,
                style: GoogleFonts.montserrat(
                  fontSize: 18,
                  fontWeight: FontWeight.bold,
                  color: Colors.black87,
                ),
              ),
              subtitle: Text(
                subtitle,
                style: GoogleFonts.openSans(
                  fontSize: 14,
                  color: Colors.black54,
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
