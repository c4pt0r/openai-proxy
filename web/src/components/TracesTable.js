import React, { useState } from 'react';

const TracesTable = ({ traces }) => {
  const [expandedRow, setExpandedRow] = useState(null); // Stores the index of the expanded row
  const [expandedSections, setExpandedSections] = useState({}); // Stores { rowIndex: { system: boolean, user: boolean, header: boolean, request: boolean, response: boolean } }

  if (!traces || traces.length === 0) {
    return <p>No valid traces available.</p>;
  }

  const toggleRow = (index) => {
    setExpandedRow(expandedRow === index ? null : index);
    if (expandedRow !== index) {
      // System, User, and Header content default to expanded (true)
      // Request and Response default to collapsed (false)
      setExpandedSections(prev => ({ ...prev, [index]: { system: true, user: true, header: true, request: false, response: false } }));
    }
  };

  const toggleSection = (rowIndex, section) => {
    setExpandedSections(prev => ({
      ...prev,
      [rowIndex]: {
        ...(prev[rowIndex] || { system: true, user: true, header: true, request: false, response: false }), // Ensure rowIndex exists with defaults
        [section]: !((prev[rowIndex] || {})[section]), // Toggle the specific section
      }
    }));
  };

  const getApiName = (url) => {
    if (!url) return "N/A";
    try {
      const pathSegments = new URL(url).pathname.split('/');
      return pathSegments.pop() || pathSegments.pop() || "N/A"; // Get last non-empty segment
    } catch (e) {
      return "N/A";
    }
  };

  const formatJson = (data) => {
    if (!data) return null;
    // Check if data is already a JS object (like http.Header will be)
    if (typeof data === 'object' && data !== null) {
      try {
        return JSON.stringify(data, null, 2);
      } catch (e) {
        // Should not happen with http.Header, but as a fallback
        return 'Error stringifying object';
      }
    }
    // If it's a string, try to parse it as JSON
    if (typeof data === 'string') {
      try {
        const parsedJson = JSON.parse(data);
        return JSON.stringify(parsedJson, null, 2);
      } catch (e) {
        // Not a valid JSON string, return as is
        return data;
      }
    }
    // For other types or if something went wrong, return a generic message
    return 'Invalid data format';
  };

  const extractContentFromRequest = (requestBody) => {
    if (!requestBody) return { systemContent: 'N/A', userContent: 'N/A' };
    try {
      const parsedBody = JSON.parse(requestBody);
      let systemContent = 'N/A';
      const userMessages = [];

      if (parsedBody.messages && Array.isArray(parsedBody.messages)) {
        parsedBody.messages.forEach(message => {
          if (message.role === 'system' && message.content) {
            systemContent = message.content;
          } else if (message.role === 'user' && message.content) {
            userMessages.push(message.content);
          }
        });
      }
      const userContent = userMessages.length > 0 ? userMessages.join('\n\n') : 'N/A';
      return { systemContent, userContent };
    } catch (e) {
      console.error("Error parsing request body for content:", e);
      return { systemContent: 'Error parsing', userContent: 'Error parsing' };
    }
  };

  return (
    <table>
      <thead>
        <tr>
          <th>Session ID</th>
          <th>Timestamp</th>
          <th>Latency</th>
          <th>API Name</th>
          <th>Details</th>
        </tr>
      </thead>
      <tbody>
        {traces.map((trace, index) => (
          <React.Fragment key={index}>
            <tr onClick={() => toggleRow(index)} style={{ cursor: 'pointer' }}>
              <td>N/A</td> {/* Session ID - Not available from backend */}
              <td>{new Date(trace.timestamp).toLocaleString()}</td>
              <td>{trace.latency ? trace.latency.toFixed(2) + 's' : "N/A"}</td>
              <td>{getApiName(trace.url)}</td>
              <td>{expandedRow === index ? '▼' : '▶'} View</td>
            </tr>
            {expandedRow === index && (
              <tr className="details-row">
                <td colSpan={5}>
                  <div className="details-content">
                    <h4 onClick={() => toggleSection(index, 'system')} style={{ cursor: 'pointer' }}>
                      System Prompt: { (expandedSections[index] && expandedSections[index].system) ? '▼' : '▶'}
                    </h4>
                    { (expandedSections[index] && expandedSections[index].system) && (
                      <pre>{extractContentFromRequest(trace.request_body).systemContent}</pre>
                    )}
                    <h4 onClick={() => toggleSection(index, 'user')} style={{ cursor: 'pointer' }}>
                      User History: { (expandedSections[index] && expandedSections[index].user) ? '▼' : '▶'}
                    </h4>
                    { (expandedSections[index] && expandedSections[index].user) && (
                      <pre>{extractContentFromRequest(trace.request_body).userContent}</pre>
                    )}
                    <h4 onClick={() => toggleSection(index, 'header')} style={{ cursor: 'pointer' }}>
                      Request Header: { (expandedSections[index] && expandedSections[index].header) ? '▼' : '▶'}
                    </h4>
                    { (expandedSections[index] && expandedSections[index].header) && (
                      <pre>{formatJson(trace.request_headers) || 'No Request Headers'}</pre>
                    )}
                    <h4 onClick={() => toggleSection(index, 'request')} style={{ cursor: 'pointer' }}>
                      Request Body: { (expandedSections[index] && expandedSections[index].request) ? '▼' : '▶'}
                    </h4>
                    { (expandedSections[index] && expandedSections[index].request) && (
                      <pre>{formatJson(trace.request_body) || 'No Request Body'}</pre>
                    )}
                    <h4 onClick={() => toggleSection(index, 'response')} style={{ cursor: 'pointer' }}>
                      Response Body: { (expandedSections[index] && expandedSections[index].response) ? '▼' : '▶'}
                    </h4>
                    { (expandedSections[index] && expandedSections[index].response) && (
                      <pre>{formatJson(trace.response_body) || 'No Response Body'}</pre>
                    )}
                  </div>
                </td>
              </tr>
            )}
          </React.Fragment>
        ))}
      </tbody>
    </table>
  );
};

export default TracesTable; 