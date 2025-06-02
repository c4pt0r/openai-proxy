import React, { useState, useEffect, useCallback } from 'react';
import './App.css';
import TracesTable from './components/TracesTable';
import SearchBar from './components/SearchBar';

function App() {
  const [traces, setTraces] = useState([]);
  const [filteredTraces, setFilteredTraces] = useState([]);
  const [searchTerm, setSearchTerm] = useState('');

  const fetchTraces = useCallback(async () => {
    try {
      const response = await fetch('/traces');
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }
      const data = await response.json();
      // The backend already returns traces in the correct order (newest first if tracesMax is working)
      // If not, you might need to sort them: data.sort((a, b) => new Date(b.timestamp) - new Date(a.timestamp));
      const validTraces = (data || []).filter(trace => trace.timestamp && trace.url);
      setTraces(validTraces);
      setFilteredTraces(validTraces);
    } catch (error) {
      console.error("Could not fetch traces:", error);
      setTraces([]); // Set to empty array on error
      setFilteredTraces([]);
    }
  }, []);

  useEffect(() => {
    fetchTraces();

    const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsHost = window.location.hostname;
    const wsPort = '8081'; // WebSocket server runs on port 8081
    const ws = new WebSocket(`${wsProtocol}//${wsHost}:${wsPort}/ws`);

    ws.onmessage = (event) => {
      try {
        const newTrace = JSON.parse(event.data);
        console.log('📨 Received WebSocket message:', newTrace);
        if (newTrace.timestamp && newTrace.url) { // Filter incoming WS traces as well
          setTraces(prevTraces => {
            const updatedTraces = [newTrace, ...prevTraces].filter(trace => trace.timestamp && trace.url).slice(0, 100);
            console.log('🔄 Updated traces, total count:', updatedTraces.length);
            return updatedTraces;
          });
        } else {
          console.log('⚠️ Received invalid trace (missing timestamp or url):', newTrace);
        }
      } catch (error) {
        console.error("Error processing WebSocket message:", error);
      }
    };

    ws.onopen = () => {
      console.log("✅ WebSocket connection established");
    };

    ws.onerror = (error) => {
      console.error("❌ WebSocket error:", error);
    };

    ws.onclose = () => {
      console.log("🔌 WebSocket connection closed");
      // Optionally, try to reconnect
    };

    return () => {
      ws.close();
    };
  }, [fetchTraces]);

  const handleSearch = useCallback(() => {
    if (!searchTerm) {
      setFilteredTraces(traces);
      return;
    }
    const lowercasedFilter = searchTerm.toLowerCase();
    const filteredData = traces.filter(item => {
      // Ensure item is valid before trying to access properties for search
      if (!item.timestamp || !item.url) return false;

      // Assuming 'id' might not exist directly on trace, or needs specific handling
      // For now, searching in URL and request/response bodies
      const urlMatch = item.url && item.url.toLowerCase().includes(lowercasedFilter);
      const requestBodyMatch = item.request_body && item.request_body.toLowerCase().includes(lowercasedFilter);
      const responseBodyMatch = item.response_body && item.response_body.toLowerCase().includes(lowercasedFilter);
      // Add more fields to search as needed, e.g., item.id, item.name (if available)
      return urlMatch || requestBodyMatch || responseBodyMatch;
    });
    setFilteredTraces(filteredData);
  }, [searchTerm, traces]);

  // Trigger search when searchTerm changes
  useEffect(() => {
    handleSearch();
  }, [searchTerm, traces, handleSearch]);


  return (
    <div className="App">
      <main>
        <SearchBar 
          searchTerm={searchTerm} 
          setSearchTerm={setSearchTerm} 
          onSearch={handleSearch} 
        />
        <TracesTable traces={filteredTraces} />
      </main>
    </div>
  );
}

export default App;
