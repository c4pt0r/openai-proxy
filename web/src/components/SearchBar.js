import React from 'react';

const SearchBar = ({ searchTerm, setSearchTerm, onSearch }) => {
  return (
    <div className="search-bar-container">
      <input
        type="text"
        placeholder="Search by id, name, user id"
        value={searchTerm}
        onChange={(e) => setSearchTerm(e.target.value)}
        onKeyPress={(e) => e.key === 'Enter' && onSearch()}
      />
      <button onClick={onSearch}>🔍</button>
      <button className="filter-button">Filter Timestamp &gt; 16/5/2024</button> 
    </div>
  );
};

export default SearchBar; 